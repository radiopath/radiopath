// Copyright (C) 2026 Fabian Berg, HB9HIL
// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/radiopath/radiopath/internal/coverage"
	"github.com/radiopath/radiopath/internal/dem"
	"github.com/radiopath/radiopath/internal/link"
	"github.com/radiopath/radiopath/internal/mail"
	"github.com/radiopath/radiopath/internal/s3"
	"github.com/radiopath/radiopath/internal/store"
	"github.com/radiopath/radiopath/internal/tiles"
	"github.com/radiopath/radiopath/internal/web"
	"github.com/radiopath/radiopath/internal/worker"
)

var version string

const minAdminToken = 32

const janitorInterval = time.Hour

const unconfirmedMaxAge = 7 * 24 * time.Hour

type config struct {
	databaseURL   string
	demDir        string
	s3            s3.Client
	s3Prefix      string
	demCacheTiles int
	canopyDir     string
	canopyPrefix  string
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
	smtp          mail.SMTP
	baseURL       string
}

func loadConfig() (config, error) {
	c := config{
		databaseURL:   os.Getenv("DATABASE_URL"),
		demDir:        os.Getenv("RADIOPATH_DEM_DIR"),
		canopyDir:     os.Getenv("RADIOPATH_CANOPY_DIR"),
		listen:        envOr("RADIOPATH_LISTEN", ":8080"),
		metricsListen: ":9090",
		redisURL:      os.Getenv("RADIOPATH_REDIS_URL"),
		tileURL:       envOr("RADIOPATH_TILE_URL", tiles.DefaultUpstream),
	}
	if v, ok := os.LookupEnv("RADIOPATH_METRICS_LISTEN"); ok {
		c.metricsListen = v
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
	c.s3 = s3.Client{
		Endpoint:  os.Getenv("RADIOPATH_DEM_S3_ENDPOINT"),
		Bucket:    os.Getenv("RADIOPATH_DEM_S3_BUCKET"),
		Region:    envOr("RADIOPATH_DEM_S3_REGION", "us-east-1"),
		AccessKey: os.Getenv("RADIOPATH_DEM_S3_ACCESS_KEY"),
		SecretKey: os.Getenv("RADIOPATH_DEM_S3_SECRET_KEY"),
	}
	c.s3Prefix = os.Getenv("RADIOPATH_DEM_S3_PREFIX")
	c.canopyPrefix = os.Getenv("RADIOPATH_CANOPY_S3_PREFIX")
	switch {
	case c.canopyPrefix != "" && c.s3.Bucket == "":
		return c, errors.New("RADIOPATH_CANOPY_S3_PREFIX needs RADIOPATH_DEM_S3_BUCKET")
	case c.s3.Bucket != "" && c.s3.Endpoint == "":
		return c, errors.New("RADIOPATH_DEM_S3_ENDPOINT is required with RADIOPATH_DEM_S3_BUCKET")
	case c.s3.Bucket != "" && (c.s3.AccessKey == "") != (c.s3.SecretKey == ""):
		return c, errors.New("RADIOPATH_DEM_S3_ACCESS_KEY and RADIOPATH_DEM_S3_SECRET_KEY must be set together")
	case c.s3.Bucket == "" && c.demDir == "":
		return c, errors.New("RADIOPATH_DEM_DIR or RADIOPATH_DEM_S3_BUCKET is required")
	}
	n, err := strconv.Atoi(envOr("RADIOPATH_DEM_CACHE_TILES", "8"))
	if err != nil || n < 1 {
		return c, errors.New("RADIOPATH_DEM_CACHE_TILES must be a positive integer")
	}
	c.demCacheTiles = n
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

func envBool(key string, def bool) (bool, error) {
	switch envOr(key, map[bool]string{true: "true", false: "false"}[def]) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	return false, errors.New(key + " must be true or false")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func newLogger(c config) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: c.logLevel}))
}

func main() {
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

func run(ctx context.Context, cfg config, log *slog.Logger) error {
	st, err := store.New(ctx, cfg.databaseURL)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.Migrate(ctx, log); err != nil {
		return err
	}

	proxy := &tiles.Proxy{Upstream: cfg.tileURL, TTL: cfg.tileTTL, Log: log}
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
	srv := &web.Server{
		Store:        st,
		Analyzer:     link.Calculator{DEM: src, Canopy: canopy},
		Tiles:        proxy,
		DEMReady:     demReady,
		Registration: cfg.registration,
		TrustProxy:   cfg.trustProxy,
		AdminToken:   cfg.adminToken,
		Version:      version,
		BaseURL:      cfg.baseURL,
		Log:          log,
	}
	if cfg.smtp.Host != "" {
		srv.Mail = cfg.smtp
		log.Info("mail", "smtp", cfg.smtp.Host, "from", cfg.smtp.From, "base_url", cfg.baseURL)
	}

	httpSrv := &http.Server{
		Addr:              cfg.listen,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	host, _ := os.Hostname()
	workerCtx, stopWorkers := context.WithCancel(ctx)
	var workers sync.WaitGroup

	workers.Add(1)
	go func() { defer workers.Done(); janitor(workerCtx, st, log) }()

	for i := 0; i < cfg.workers; i++ {
		w := &worker.Worker{Name: fmt.Sprintf("%s/%d", host, i), Store: st, Coverage: coverage.Calculator{DEM: src}, Log: log}
		workers.Add(1)
		go func() { defer workers.Done(); w.Run(workerCtx) }()
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
		if _, err := st.DeleteExpiredEmailTokens(ctx); err != nil {
			log.Warn("email token cleanup", "err", err)
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
