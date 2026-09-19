package tiles

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type memCache struct {
	mu sync.Mutex
	m  map[string][]byte
	n  map[string]int
}

func (c *memCache) Get(_ context.Context, k string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.m[k], nil
}
func (c *memCache) Set(_ context.Context, k string, v []byte, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[k] = v
	return nil
}
func (c *memCache) Incr(_ context.Context, k string, _ time.Duration) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.n == nil {
		c.n = map[string]int{}
	}
	c.n[k]++
	return int64(c.n[k]), nil
}

func (c *memCache) Ping(context.Context) error { return nil }

func newProxy(t *testing.T, upstream http.HandlerFunc, cache Cache) (*Proxy, *int) {
	t.Helper()
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		upstream(w, r)
	}))
	t.Cleanup(srv.Close)
	return &Proxy{Upstream: srv.URL + "/{z}/{x}/{y}.png", Cache: cache, TTL: time.Hour, Log: slog.Default()}, &hits
}

func serve(p *Proxy, path string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.Handle("GET /tiles/{z}/{x}/{y}", p)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestProxyCaches(t *testing.T) {
	png := append([]byte("\x89PNG\r\n\x1a\n"), []byte("tile")...)
	var gotUA string
	p, hits := newProxy(t, func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.UserAgent()
		if r.URL.Path != "/5/16/10.png" {
			t.Errorf("upstream path %s", r.URL.Path)
		}
		w.Write(png)
	}, &memCache{m: map[string][]byte{}})

	for i, want := range []string{"MISS", "HIT"} {
		rec := serve(p, "/tiles/5/16/10.png")
		if rec.Code != 200 || rec.Header().Get("X-Cache") != want || rec.Header().Get("Content-Type") != "image/png" {
			t.Errorf("request %d: code %d, %v", i, rec.Code, rec.Header())
		}
		if rec.Header().Get("Cache-Control") != "public, max-age=3600" {
			t.Errorf("cache-control %q", rec.Header().Get("Cache-Control"))
		}
		if rec.Body.String() != string(png) {
			t.Errorf("body mismatch")
		}
	}
	if *hits != 1 {
		t.Errorf("upstream hit %d times, want 1", *hits)
	}
	if gotUA != userAgent {
		t.Errorf("user agent %q", gotUA)
	}
}

func TestProxyETag(t *testing.T) {
	png := []byte("\x89PNG-etag")
	p, hits := newProxy(t, func(w http.ResponseWriter, _ *http.Request) { w.Write(png) }, &memCache{m: map[string][]byte{}})
	rec := serve(p, "/tiles/2/1/1.png")
	etag := rec.Header().Get("ETag")
	if rec.Code != 200 || etag == "" || etag != ETag(png) {
		t.Fatalf("code %d etag %q", rec.Code, etag)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /tiles/{z}/{x}/{y}", p)
	req := httptest.NewRequest(http.MethodGet, "/tiles/2/1/1.png", nil)
	req.Header.Set("If-None-Match", etag)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 304 || rec.Body.Len() != 0 || rec.Header().Get("ETag") != etag || rec.Header().Get("Cache-Control") == "" {
		t.Errorf("conditional request: code %d body %d headers %v", rec.Code, rec.Body.Len(), rec.Header())
	}
	if *hits != 1 {
		t.Errorf("upstream hits %d, want 1", *hits)
	}
}

func TestProxyValidation(t *testing.T) {
	p, hits := newProxy(t, func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("\x89PNG")) }, nil)
	for _, path := range []string{"/tiles/20/0/0.png", "/tiles/3/8/0.png", "/tiles/3/0/-1.png", "/tiles/a/0/0.png", "/tiles/3/0/0.jpg", "/tiles/3/0/0"} {
		if rec := serve(p, path); rec.Code != 404 {
			t.Errorf("%s: code %d", path, rec.Code)
		}
	}
	if *hits != 0 {
		t.Errorf("invalid requests reached upstream")
	}
}

func TestProxyUpstreamErrors(t *testing.T) {
	p, _ := newProxy(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(404) }, nil)
	if rec := serve(p, "/tiles/1/0/0.png"); rec.Code != 502 || rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("upstream 404: code %d, %v", rec.Code, rec.Header())
	}
	p, _ = newProxy(t, func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("<html>")) }, nil)
	if rec := serve(p, "/tiles/1/0/0.png"); rec.Code != 502 {
		t.Errorf("non-PNG: code %d", rec.Code)
	}
}

func TestProxyClientCanceled(t *testing.T) {
	p, _ := newProxy(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write(append([]byte("\x89PNG\r\n\x1a\n"), []byte("tile")...))
	}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mux := http.NewServeMux()
	mux.Handle("GET /tiles/{z}/{x}/{y}", p)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/tiles/1/0/0.png", nil).WithContext(ctx))
	if rec.Code != statusClientClosed || rec.Body.Len() != 0 {
		t.Errorf("cancelled request: code %d, body %q", rec.Code, rec.Body.String())
	}
}

func TestUpstreamURL(t *testing.T) {
	p := &Proxy{Upstream: "https://{s}.example.org/{z}/{x}/{y}{r}.png"}
	if got := p.upstreamURL(3, 4, 5); got != "https://a.example.org/3/4/5.png" {
		t.Errorf("url %s", got)
	}
	if k := p.key(3, 4, 5); len(k) != len("tile:12345678:3/4/5") {
		t.Errorf("key %s", k)
	}
}

func TestStatic(t *testing.T) {
	tile := func(c color.RGBA) []byte {
		img := image.NewRGBA(image.Rect(0, 0, 256, 256))
		draw.Draw(img, img.Bounds(), &image.Uniform{c}, image.Point{}, draw.Src)
		var b bytes.Buffer
		png.Encode(&b, img)
		return b.Bytes()
	}
	red, blue := tile(color.RGBA{255, 0, 0, 255}), tile(color.RGBA{0, 0, 255, 255})
	p, hits := newProxy(t, func(w http.ResponseWriter, r *http.Request) {
		var z, x, y int
		fmt.Sscanf(r.URL.Path, "/%d/%d/%d.png", &z, &x, &y)
		if x%2 == 0 {
			w.Write(red)
		} else {
			w.Write(blue)
		}
	}, &memCache{m: map[string][]byte{}})

	v := Fit(47.0, 9.0, 47.5, 9.7, 700, 400, 40)
	if v.W != 700 || v.H != 400 || v.Zoom < 8 || v.Zoom > 10 {
		t.Fatalf("view %+v", v)
	}
	x1, y1 := v.Pixel(47.5, 9.0)
	x2, y2 := v.Pixel(47.0, 9.7)
	if x1 < 40 || y1 < 40 || x2 > 660 || y2 > 360 || math.Abs((x1+x2)/2-350) > 1 || math.Abs((y1+y2)/2-200) > 1 {
		t.Errorf("bounds at %.0f,%.0f - %.0f,%.0f", x1, y1, x2, y2)
	}

	out, err := p.Static(context.Background(), v)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil || img.Bounds().Dx() != 700 || img.Bounds().Dy() != 400 {
		t.Fatalf("image %v %v", img.Bounds(), err)
	}
	seen := map[color.RGBA]bool{}
	for _, pt := range []image.Point{{0, 0}, {699, 399}, {350, 200}, {100, 300}} {
		c := color.RGBAModel.Convert(img.At(pt.X, pt.Y)).(color.RGBA)
		if c.A != 255 || (c.R != 255 && c.B != 255) {
			t.Errorf("pixel %v = %v", pt, c)
		}
		seen[c] = true
	}
	if len(seen) != 2 {
		t.Errorf("tile columns %v", seen)
	}
	if *hits > 12 {
		t.Errorf("%d upstream fetches for a 700x400 map", *hits)
	}
	n := *hits
	if _, err := p.Static(context.Background(), v); err != nil || *hits != n {
		t.Errorf("cache: %v, %d more fetches", err, *hits-n)
	}
}

func TestTrim(t *testing.T) {
	v := Fit(47.0, 9.0, 47.5, 9.7, 2400, 2400, 60)
	tr := v.Trim(47.0, 9.0, 47.5, 9.7, 60)
	x1, y1 := tr.Pixel(47.5, 9.0)
	x2, y2 := tr.Pixel(47.0, 9.7)
	if tr.Zoom != v.Zoom || tr.W > v.W || tr.H > v.H {
		t.Errorf("trim grew or changed zoom: %+v -> %+v", v, tr)
	}
	if x1 < 59 || x1 > 61 || y1 < 59 || y1 > 61 || float64(tr.W)-x2 < 59 || float64(tr.W)-x2 > 62 || float64(tr.H)-y2 < 59 || float64(tr.H)-y2 > 62 {
		t.Errorf("bounds at %.0f,%.0f - %.0f,%.0f in %dx%d", x1, y1, x2, y2, tr.W, tr.H)
	}
	if mpp := tr.MetresPerPixel(47.5); math.Abs((x2-x1)*mpp/0.7-75360) > 500 {
		t.Errorf("metres per pixel %.2f gives %.0f m per degree", mpp, (x2-x1)*mpp/0.7)
	}
}

func TestMissLimit(t *testing.T) {
	png := append([]byte("\x89PNG\r\n\x1a\n"), []byte("tile")...)
	p, hits := newProxy(t, func(w http.ResponseWriter, _ *http.Request) { w.Write(png) }, &memCache{m: map[string][]byte{}})
	p.MissesPerIP = 2
	p.ClientIP = func(*http.Request) string { return "198.51.100.7" }

	for i, want := range []int{200, 200, 429} {
		if rec := serve(p, fmt.Sprintf("/tiles/5/%d/0.png", i)); rec.Code != want {
			t.Fatalf("tile %d: status %d, want %d", i, rec.Code, want)
		}
	}
	if *hits != 2 {
		t.Errorf("upstream fetches = %d, want 2", *hits)
	}
	// a cached tile costs no budget, so it is still served while the client is throttled
	if rec := serve(p, "/tiles/5/0/0.png"); rec.Code != 200 || rec.Header().Get("X-Cache") != "HIT" {
		t.Errorf("cached tile: status %d, X-Cache %q", rec.Code, rec.Header().Get("X-Cache"))
	}
}
