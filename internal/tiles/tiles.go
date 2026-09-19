package tiles

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/radiopath/radiopath/internal/metrics"
)

const (
	DefaultUpstream    = "https://tile.openstreetmap.org/{z}/{x}/{y}.png"
	DefaultTTL         = 30 * 24 * time.Hour
	DefaultMissesPerIP = 300
	maxZoom            = 19
	maxTileBytes       = 2 << 20
	userAgent          = "RadiopathTileProxy/1.0 (github.com/radiopath/radiopath)"
	statusClientClosed = 499 // Leaflet aborts tiles on pan and zoom; like nginx, not a 5xx
)

var pngMagic = []byte("\x89PNG")

type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, val []byte, ttl time.Duration) error
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
	Ping(ctx context.Context) error
}

type RedisCache struct {
	c *redis.Client
}

func NewRedisCache(url string) (*RedisCache, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	return &RedisCache{c: redis.NewClient(opt)}, nil
}

func (r *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	b, err := r.c.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	return b, err
}

func (r *RedisCache) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	return r.c.Set(ctx, key, val, ttl).Err()
}

func (r *RedisCache) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	n, err := r.c.Incr(ctx, key).Result()
	if err == nil && n == 1 {
		err = r.c.Expire(ctx, key, ttl).Err()
	}
	return n, err
}

func (r *RedisCache) Ping(ctx context.Context) error { return r.c.Ping(ctx).Err() }

func (r *RedisCache) Close() error { return r.c.Close() }

type Proxy struct {
	Upstream string
	Cache    Cache
	TTL      time.Duration
	Log      *slog.Logger

	MissesPerIP int
	ClientIP    func(*http.Request) string

	Client *http.Client
	prefix string
}

const missWindow = time.Minute

var errLimited = errors.New("tile miss budget exhausted")

func (p *Proxy) client() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: 8 * time.Second}
}

func (p *Proxy) key(z, x, y int) string {
	if p.prefix == "" {
		sum := sha1.Sum([]byte(p.Upstream))
		p.prefix = "tile:" + hex.EncodeToString(sum[:4]) + ":"
	}
	return fmt.Sprintf("%s%d/%d/%d", p.prefix, z, x, y)
}

func (p *Proxy) upstreamURL(z, x, y int) string {
	r := strings.NewReplacer(
		"{z}", strconv.Itoa(z), "{x}", strconv.Itoa(x), "{y}", strconv.Itoa(y),
		"{s}", string("abc"[(x+y)%3]), "{r}", "")
	return r.Replace(p.Upstream)
}

func (p *Proxy) Ready(ctx context.Context) error {
	if p.Cache == nil {
		return nil
	}
	return p.Cache.Ping(ctx)
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	yPng, ok := strings.CutSuffix(r.PathValue("y"), ".png")
	z, errZ := strconv.Atoi(r.PathValue("z"))
	x, errX := strconv.Atoi(r.PathValue("x"))
	y, errY := strconv.Atoi(yPng)
	if !ok || errZ != nil || errX != nil || errY != nil || z < 0 || z > maxZoom || x < 0 || y < 0 || x >= 1<<z || y >= 1<<z {
		http.NotFound(w, r)
		return
	}
	png, status, err := p.tile(r.Context(), p.clientIP(r), z, x, y)
	if err != nil {
		if errors.Is(err, errLimited) {
			w.Header().Set("Retry-After", strconv.Itoa(int(missWindow.Seconds())))
			http.Error(w, "too many tiles", http.StatusTooManyRequests)
			return
		}
		if r.Context().Err() != nil {
			w.WriteHeader(statusClientClosed)
			return
		}
		p.Log.Warn("tile upstream", "z", z, "x", x, "y", y, "err", err)
		w.Header().Set("Cache-Control", "no-store")
		http.Error(w, "tile unavailable", http.StatusBadGateway)
		return
	}
	p.write(w, r, png, status)
}

func (p *Proxy) clientIP(r *http.Request) string {
	if p.ClientIP == nil {
		return ""
	}
	return p.ClientIP(r)
}

func (p *Proxy) allow(ctx context.Context, ip string) bool {
	if p.MissesPerIP <= 0 || p.Cache == nil || ip == "" {
		return true
	}
	n, err := p.Cache.Incr(ctx, p.prefix+"limit:"+ip, missWindow)
	if err != nil {
		p.Log.Warn("tile limit", "err", err)
		return true
	}
	return n <= int64(p.MissesPerIP)
}

func (p *Proxy) tile(ctx context.Context, ip string, z, x, y int) ([]byte, string, error) {
	key := p.key(z, x, y)
	if p.Cache != nil {
		png, err := p.Cache.Get(ctx, key)
		if err != nil {
			p.Log.Warn("tile cache get", "err", err)
		} else if png != nil {
			metrics.TileRequests.WithLabelValues("hit").Inc()
			return png, "HIT", nil
		}
	}
	if !p.allow(ctx, ip) {
		metrics.TileRequests.WithLabelValues("limited").Inc()
		return nil, "", errLimited
	}
	png, err := p.fetch(ctx, z, x, y)
	if err != nil {
		result := "error"
		if ctx.Err() != nil {
			result = "canceled"
		}
		metrics.TileRequests.WithLabelValues(result).Inc()
		return nil, "", err
	}
	if p.Cache != nil {
		if err := p.Cache.Set(ctx, key, png, p.TTL); err != nil {
			p.Log.Warn("tile cache set", "err", err)
		}
	}
	metrics.TileRequests.WithLabelValues("miss").Inc()
	return png, "MISS", nil
}

func (p *Proxy) fetch(ctx context.Context, z, x, y int) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.upstreamURL(z, x, y), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := p.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxTileBytes {
		return nil, errors.New("tile too large")
	}
	if !bytes.HasPrefix(body, pngMagic) {
		return nil, errors.New("not a PNG")
	}
	return body, nil
}

func (p *Proxy) write(w http.ResponseWriter, r *http.Request, png []byte, status string) {
	etag := ETag(png)
	h := w.Header()
	h.Set("Cache-Control", "public, max-age="+strconv.Itoa(int(p.TTL.Seconds())))
	h.Set("ETag", etag)
	h.Set("X-Cache", status)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	h.Set("Content-Type", "image/png")
	w.Write(png)
}

func ETag(b []byte) string {
	sum := sha256.Sum256(b)
	return `"` + hex.EncodeToString(sum[:8]) + `"`
}
