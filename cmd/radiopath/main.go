// Copyright (C) 2026 Fabian Berg, HB9HIL
// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/radiopath/radiopath/internal/coverage"
	"github.com/radiopath/radiopath/internal/dem"
	"github.com/radiopath/radiopath/internal/hf"
	"github.com/radiopath/radiopath/internal/link"
	"github.com/radiopath/radiopath/internal/mail"
	"github.com/radiopath/radiopath/internal/s3"
	"github.com/radiopath/radiopath/internal/store"
	"github.com/radiopath/radiopath/internal/sunspot"
	"github.com/radiopath/radiopath/internal/tiles"
	"github.com/radiopath/radiopath/internal/web"
	"github.com/radiopath/radiopath/internal/webui"
	"github.com/radiopath/radiopath/internal/worker"
)

var version string

const minAdminToken = 32

const janitorInterval = time.Hour

const unconfirmedMaxAge = 7 * 24 * time.Hour

// expired API tokens stay listed this long, so their owner sees why a client stopped working
const expiredTokenKeep = 30 * 24 * time.Hour

type config struct {
	databaseURL   string
	demDir        string
	s3            s3.Client
	s3Prefix      string
	demCacheTiles int
	canopyDir     string
	canopyPrefix  string
	hfDir         string // the HF data set folder, empty when there is none
	hfPrefix      string // its key prefix in the S3 bucket, empty for HF off
	sunspotURL    string // NOAA SWPC solar cycle files for the HF sunspot number; empty turns the download off
	listen        string
	metricsListen string
	logLevel      slog.Level
	registration  bool
	trustProxy    bool
	adminToken    string
	workers       int
	redisURL      string
	tileURL       string
	tileTTL       time.Duration
	tileMisses    int
	smtp          mail.SMTP
	baseURL       string
}

// loadConfig reads the RADIOPATH_* and DATABASE_URL environment into a config; it rejects registration without SMTP.
func loadConfig() (config, error) {
	c := config{
		databaseURL:   os.Getenv("DATABASE_URL"),
		listen:        envOr("RADIOPATH_LISTEN", ":8080"),
		metricsListen: ":9090",
		redisURL:      os.Getenv("RADIOPATH_REDIS_URL"),
		tileURL:       envOr("RADIOPATH_TILE_URL", tiles.DefaultUpstream),
	}
	if v, ok := os.LookupEnv("RADIOPATH_METRICS_LISTEN"); ok {
		c.metricsListen = v
	}
	c.sunspotURL = sunspot.DefaultURL
	if v, ok := os.LookupEnv("RADIOPATH_SUNSPOT_URL"); ok {
		c.sunspotURL = v
	}
	ttl, err := time.ParseDuration(envOr("RADIOPATH_TILE_TTL", tiles.DefaultTTL.String()))
	if err != nil || ttl <= 0 {
		return c, errors.New("RADIOPATH_TILE_TTL must be a positive duration like 720h")
	}
	c.tileTTL = ttl
	var err2 error
	if c.registration, err2 = envBool("RADIOPATH_REGISTRATION", true); err2 != nil {
		return c, err2
	}
	if c.trustProxy, err2 = envBool("RADIOPATH_TRUST_PROXY", false); err2 != nil {
		return c, err2
	}
	if c.databaseURL == "" {
		return c, errors.New("DATABASE_URL is required")
	}
	c.adminToken = os.Getenv("RADIOPATH_ADMIN_TOKEN")
	if c.adminToken != "" && len(c.adminToken) < minAdminToken {
		return c, fmt.Errorf("RADIOPATH_ADMIN_TOKEN must be at least %d characters", minAdminToken)
	}
	if err := datasets(&c); err != nil {
		return c, err
	}
	n, err := strconv.Atoi(envOr("RADIOPATH_DEM_CACHE_TILES", "8"))
	if err != nil || n < 1 {
		return c, errors.New("RADIOPATH_DEM_CACHE_TILES must be a positive integer")
	}
	c.demCacheTiles = n
	c.tileMisses, err = strconv.Atoi(envOr("RADIOPATH_TILE_MISSES_PER_IP", strconv.Itoa(tiles.DefaultMissesPerIP)))
	if err != nil || c.tileMisses < 0 {
		return c, errors.New("RADIOPATH_TILE_MISSES_PER_IP must be 0 or a positive integer")
	}
	c.workers, err = strconv.Atoi(envOr("RADIOPATH_WORKERS", "1"))
	if err != nil || c.workers < 0 {
		return c, errors.New("RADIOPATH_WORKERS must be 0 or a positive integer")
	}
	if err := c.logLevel.UnmarshalText([]byte(envOr("RADIOPATH_LOG_LEVEL", "info"))); err != nil {
		return c, errors.New("RADIOPATH_LOG_LEVEL must be debug, info, warn or error")
	}
	c.smtp = mail.SMTP{
		Host:     os.Getenv("RADIOPATH_SMTP_HOST"),
		User:     os.Getenv("RADIOPATH_SMTP_USER"),
		Password: os.Getenv("RADIOPATH_SMTP_PASSWORD"),
		From:     os.Getenv("RADIOPATH_MAIL_FROM"),
	}
	c.baseURL = strings.TrimSuffix(os.Getenv("RADIOPATH_BASE_URL"), "/")
	if c.smtp.Host != "" {
		if c.smtp.Port, err = strconv.Atoi(envOr("RADIOPATH_SMTP_PORT", "587")); err != nil || c.smtp.Port < 1 {
			return c, errors.New("RADIOPATH_SMTP_PORT must be a port number")
		}
		if c.smtp.From == "" {
			return c, errors.New("RADIOPATH_MAIL_FROM is required with RADIOPATH_SMTP_HOST")
		}
		starttls, err := envBool("RADIOPATH_SMTP_STARTTLS", true)
		if err != nil {
			return c, err
		}
		c.smtp.Plain = !starttls
		if u, err := url.Parse(c.baseURL); c.baseURL == "" || err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return c, errors.New("RADIOPATH_BASE_URL must be an absolute http(s) URL when mail is configured")
		}
	}
	if c.registration && c.smtp.Host == "" {
		return c, errors.New("RADIOPATH_REGISTRATION needs RADIOPATH_SMTP_HOST: new accounts confirm their address by mail")
	}
	return c, nil
}

// replaced lists the dataset variables of earlier releases and what took their place; a server still configured
// with one refuses to start rather than run without terrain.
var replaced = [][2]string{
	{"RADIOPATH_DEM_DIR", "RADIOPATH_DATASET_DIR"},
	{"RADIOPATH_DEM_S3_ENDPOINT", "RADIOPATH_DATASET_S3_ENDPOINT"},
	{"RADIOPATH_DEM_S3_BUCKET", "RADIOPATH_DATASET_S3_BUCKET"},
	{"RADIOPATH_DEM_S3_PREFIX", "RADIOPATH_DATASET_DEM"},
	{"RADIOPATH_DEM_S3_REGION", "RADIOPATH_DATASET_S3_REGION"},
	{"RADIOPATH_DEM_S3_ACCESS_KEY", "RADIOPATH_DATASET_S3_ACCESS_KEY"},
	{"RADIOPATH_DEM_S3_SECRET_KEY", "RADIOPATH_DATASET_S3_SECRET_KEY"},
	{"RADIOPATH_CANOPY_DIR", "RADIOPATH_DATASET_CANOPY"},
	{"RADIOPATH_CANOPY_S3_PREFIX", "RADIOPATH_DATASET_CANOPY"},
	{"RADIOPATH_HF_DIR", "RADIOPATH_DATASET_HF"},
}

// datasets fills in where the terrain, canopy and HF data are: one root, a directory or an S3 bucket, with a folder
// (key prefix) per dataset. An empty RADIOPATH_DATASET_CANOPY or RADIOPATH_DATASET_HF, or a missing folder, means no
// vegetation model or no HF model.
func datasets(c *config) error {
	for _, r := range replaced {
		if _, ok := os.LookupEnv(r[0]); ok {
			return fmt.Errorf("%s is no longer read; use %s (see docs/reference/configuration.md)", r[0], r[1])
		}
	}
	demName := envOr("RADIOPATH_DATASET_DEM", "srtm1")
	canopyName, ok := os.LookupEnv("RADIOPATH_DATASET_CANOPY")
	if !ok {
		canopyName = "canopy1"
	}
	hfName, ok := os.LookupEnv("RADIOPATH_DATASET_HF")
	if !ok {
		hfName = "hf1"
	}
	c.s3 = s3.Client{
		Endpoint:  os.Getenv("RADIOPATH_DATASET_S3_ENDPOINT"),
		Bucket:    os.Getenv("RADIOPATH_DATASET_S3_BUCKET"),
		Region:    envOr("RADIOPATH_DATASET_S3_REGION", "us-east-1"),
		AccessKey: os.Getenv("RADIOPATH_DATASET_S3_ACCESS_KEY"),
		SecretKey: os.Getenv("RADIOPATH_DATASET_S3_SECRET_KEY"),
	}
	root := os.Getenv("RADIOPATH_DATASET_DIR")
	switch {
	case c.s3.Bucket != "" && c.s3.Endpoint == "":
		return errors.New("RADIOPATH_DATASET_S3_ENDPOINT is required with RADIOPATH_DATASET_S3_BUCKET")
	case c.s3.Bucket != "" && (c.s3.AccessKey == "") != (c.s3.SecretKey == ""):
		return errors.New("RADIOPATH_DATASET_S3_ACCESS_KEY and RADIOPATH_DATASET_S3_SECRET_KEY must be set together")
	case c.s3.Bucket != "":
		c.s3Prefix = demName + "/"
		if canopyName != "" {
			c.canopyPrefix = canopyName + "/"
		}
		if hfName != "" {
			c.hfPrefix = hfName + "/"
		}
	case root == "":
		return errors.New("RADIOPATH_DATASET_DIR or RADIOPATH_DATASET_S3_BUCKET is required")
	default:
		c.demDir = filepath.Join(root, demName)
		if fi, err := os.Stat(filepath.Join(root, canopyName)); canopyName != "" && err == nil && fi.IsDir() {
			c.canopyDir = filepath.Join(root, canopyName)
		}
		if fi, err := os.Stat(filepath.Join(root, hfName)); hfName != "" && err == nil && fi.IsDir() {
			c.hfDir = filepath.Join(root, hfName)
		}
	}
	return nil
}

// envBool parses key as true/false, falling back to def when unset.
func envBool(key string, def bool) (bool, error) {
	switch envOr(key, map[bool]string{true: "true", false: "false"}[def]) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	return false, errors.New(key + " must be true or false")
}

// envOr returns the environment variable key, or def when it is unset or empty.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// newLogger returns a JSON slog logger on stderr at the configured level.
func newLogger(c config) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: c.logLevel}))
}

// main prints the OpenAPI description or runs a user subcommand when given arguments, otherwise the server until SIGINT/SIGTERM.
func main() {
	if len(os.Args) == 2 && os.Args[1] == "openapi" {
		spec, err := (&web.Server{}).OpenAPI()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Stdout.Write(append(spec, '\n'))
		return
	}
	if len(os.Args) > 1 {
		os.Exit(userCommand(os.Args[1:]))
	}
	cfg, err := loadConfig()
	if err != nil {
		newLogger(config{}).Error("config", "err", err)
		os.Exit(2)
	}
	log := newLogger(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg, log); err != nil {
		log.Error("exit", "err", err)
		os.Exit(1)
	}
}

// openHF opens the HF data set: its folder in the dataset directory, or the files under its key prefix in the S3
// bucket, downloaded and verified into a temporary directory first (about 130 MB). The returned func removes that
// directory. Without a folder, or with the files missing from the bucket, the HF model stays off and the result is nil;
// an error also leaves it off, run only logs it so the rest of the app still starts.
func openHF(ctx context.Context, cfg config, log *slog.Logger) (*hf.Data, func(), error) {
	dir, cleanup := cfg.hfDir, func() {}
	if cfg.s3.Bucket != "" && cfg.hfPrefix != "" {
		tmp, err := os.MkdirTemp("", "radiopath-hf-")
		if err != nil {
			return nil, cleanup, err
		}
		cleanup = func() { os.RemoveAll(tmp) }
		err = hf.Fetch(ctx, tmp, func(ctx context.Context, name string) (io.ReadCloser, error) {
			return cfg.s3.Get(ctx, cfg.hfPrefix+name)
		})
		if errors.Is(err, s3.ErrNotFound) {
			log.Warn("hf data set is not in the bucket, the HF model is off", "err", err)
			return nil, cleanup, nil
		}
		if err != nil {
			return nil, cleanup, err
		}
		dir = tmp
	}
	if dir == "" {
		return nil, cleanup, nil
	}
	d, err := hf.Open(dir, "")
	if err == nil {
		err = d.Ready()
	}
	if err != nil {
		return nil, cleanup, fmt.Errorf("hf data: %w", err)
	}
	log.Info("hf data", "dir", dir, "s3", cfg.hfPrefix != "" && cfg.s3.Bucket != "")
	return d, cleanup, nil
}

// run starts the store, workers, janitor, metrics and HTTP server, and shuts them down in order on cancel or listen error.
func run(ctx context.Context, cfg config, log *slog.Logger) error {
	log.Info("starting", "version", version, "workers", cfg.workers, "listen", cfg.listen, "metrics", cfg.metricsListen)
	st, err := store.New(ctx, cfg.databaseURL)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.Migrate(ctx, log); err != nil {
		return err
	}

	proxy := &tiles.Proxy{Upstream: cfg.tileURL, TTL: cfg.tileTTL, MissesPerIP: cfg.tileMisses, Log: log}
	if cfg.redisURL != "" {
		rc, err := tiles.NewRedisCache(cfg.redisURL)
		if err != nil {
			return err
		}
		defer rc.Close()
		proxy.Cache = rc
	} else {
		log.Warn("RADIOPATH_REDIS_URL not set, map tiles are not cached")
	}

	var src dem.Source
	demReady := func() error { return nil }
	if cfg.s3.Bucket != "" {
		src = dem.NewS3Source(&cfg.s3, cfg.s3Prefix, cfg.demCacheTiles)
		log.Info("dem source", "s3", cfg.s3.Endpoint, "bucket", cfg.s3.Bucket, "prefix", cfg.s3Prefix, "signed", cfg.s3.AccessKey != "")
	} else {
		d := dem.NewDirSource(cfg.demDir, cfg.demCacheTiles)
		src, demReady = d, d.Ready
		log.Info("dem source", "dir", cfg.demDir)
	}
	var canopy dem.Canopy
	switch {
	case cfg.s3.Bucket != "" && cfg.canopyPrefix != "":
		canopy = dem.NewS3Canopy(&cfg.s3, cfg.canopyPrefix, cfg.demCacheTiles)
		log.Info("canopy source", "s3", cfg.s3.Endpoint, "bucket", cfg.s3.Bucket, "prefix", cfg.canopyPrefix)
	case cfg.canopyDir != "":
		if _, err := os.ReadDir(cfg.canopyDir); err != nil {
			return fmt.Errorf("canopy dir: %w", err)
		}
		canopy = dem.NewDirCanopy(cfg.canopyDir, cfg.demCacheTiles)
		log.Info("canopy source", "dir", cfg.canopyDir)
	}
	hfData, closeHF, err := openHF(ctx, cfg, log)
	defer closeHF()
	if err != nil {
		log.Error("hf data set unusable, the HF model is off", "err", err)
	}
	var sunspots *sunspot.Source
	if hfData != nil && cfg.sunspotURL != "" {
		sunspots = sunspot.New(cfg.sunspotURL)
	}
	srv := &web.Server{
		Store:        st,
		HF:           hfData,
		Sunspots:     sunspots,
		Analyzer:     link.Calculator{DEM: src, Canopy: canopy},
		Tiles:        proxy,
		DEMReady:     demReady,
		Registration: cfg.registration,
		TrustProxy:   cfg.trustProxy,
		AdminToken:   cfg.adminToken,
		Version:      version,
		BaseURL:      cfg.baseURL,
		UI:           webui.FS(),
		Log:          log,
	}
	if cfg.smtp.Host != "" {
		srv.Mail = cfg.smtp
		log.Info("mail service started", "smtp", cfg.smtp.Host, "from", cfg.smtp.From, "base_url", cfg.baseURL)
	}

	httpSrv := &http.Server{
		Addr:              cfg.listen,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		// the whole request, so a slowly trickled body cannot hold a connection; no WriteTimeout, it would cut the
		// job event streams
		ReadTimeout: 30 * time.Second,
		IdleTimeout: 2 * time.Minute,
	}

	host, _ := os.Hostname()
	workerCtx, stopWorkers := context.WithCancel(ctx)
	var workers sync.WaitGroup

	workers.Add(1)
	go func() { defer workers.Done(); janitor(workerCtx, st, log) }()

	if sunspots != nil {
		workers.Add(1)
		go func() { defer workers.Done(); sunspots.Run(workerCtx, log) }()
	}

	// Notifications wake the workers and the requests that follow a job; both also poll in case one is lost.
	wakeCoverage, wakeShort := make(chan struct{}, 1), make(chan struct{}, 1)
	workers.Add(2)
	go func() {
		defer workers.Done()
		st.Listen(workerCtx, store.ChanJobQueued, log, func(kind string) {
			wake := wakeShort
			if kind == store.KindCoverage {
				wake = wakeCoverage
			}
			select {
			case wake <- struct{}{}:
			default:
			}
		})
	}()
	go func() { defer workers.Done(); st.Listen(workerCtx, store.ChanJobDone, log, srv.JobDone) }()

	for i := 0; i < cfg.workers; i++ {
		w := &worker.Worker{Name: fmt.Sprintf("%s/%d", host, i), Store: st, Coverage: coverage.Calculator{DEM: src}, Log: log, Wake: wakeCoverage}
		workers.Add(1)
		go func() { defer workers.Done(); w.Run(workerCtx) }()
	}
	// Short jobs (link analyses) run next to the coverage workers, one per CPU; a process without coverage workers
	// computes nothing.
	if cfg.workers > 0 {
		for i := 0; i < runtime.GOMAXPROCS(0); i++ {
			w := &worker.Worker{Name: fmt.Sprintf("%s/short/%d", host, i), Store: st, Compute: srv.RunJob, Log: log, Wake: wakeShort}
			workers.Add(1)
			go func() { defer workers.Done(); w.Run(workerCtx) }()
		}
	}

	var metricsSrv *http.Server
	if cfg.metricsListen != "" {
		metricsSrv = metricsServer(cfg.metricsListen, st, log)
		go func() {
			log.Info("metrics listening", "addr", cfg.metricsListen)
			if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error("metrics server", "err", err)
			}
		}()
	}

	errc := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.listen, "workers", cfg.workers, "version", version)
		errc <- httpSrv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		stopWorkers()
		workers.Wait()
		return err
	case <-ctx.Done():
	}

	log.Info("shutting down")
	stopWorkers()
	workers.Wait()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = httpSrv.Shutdown(shutdownCtx)
	if metricsSrv != nil {
		metricsSrv.Close()
	}
	return err
}

// finishedJobKeep is how long the result of a short job stays as the cache of its input.
const finishedJobKeep = time.Hour

// janitor periodically removes expired sessions, rate limits, email tokens, long expired API tokens, unconfirmed users and finished short jobs until ctx ends.
func janitor(ctx context.Context, st *store.Store, log *slog.Logger) {
	tick := time.NewTicker(janitorInterval)
	defer tick.Stop()
	for {
		if n, err := st.DeleteExpiredSessions(ctx); err != nil {
			log.Warn("session cleanup", "err", err)
		} else if n > 0 {
			log.Info("expired sessions removed", "count", n)
		}
		if err := st.DeleteOldRateLimits(ctx); err != nil {
			log.Warn("rate limit cleanup", "err", err)
		}
		if _, err := st.DeleteFinishedJobs(ctx, finishedJobKeep); err != nil {
			log.Warn("job cleanup", "err", err)
		}
		if _, err := st.DeleteExpiredEmailTokens(ctx); err != nil {
			log.Warn("email token cleanup", "err", err)
		}
		if _, err := st.DeleteExpiredTokens(ctx, expiredTokenKeep); err != nil {
			log.Warn("API token cleanup", "err", err)
		}
		if n, err := st.DeleteUnconfirmedUsers(ctx, unconfirmedMaxAge); err != nil {
			log.Warn("unconfirmed account cleanup", "err", err)
		} else if n > 0 {
			log.Info("unconfirmed accounts removed", "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
