package dem

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/radiopath/radiopath/internal/metrics"
	"github.com/radiopath/radiopath/internal/s3"
)

type s3Loader struct {
	client  *s3.Client
	prefix  string
	f       format
	timeout time.Duration
}

func (l s3Loader) key(name string) string { return l.prefix + name[:3] + "/" + name + l.f.ext }

func (l s3Loader) load(name string) (*tile, error) {
	ctx, cancel := context.WithTimeout(context.Background(), l.timeout)
	defer cancel()
	body, err := l.client.Get(ctx, l.key(name))
	if errors.Is(err, s3.ErrNotFound) {
		metrics.S3Requests.WithLabelValues("notfound").Inc()
		return nil, fmt.Errorf("%w: %s", ErrNoTile, name)
	}
	if err != nil {
		metrics.S3Requests.WithLabelValues("error").Inc()
		return nil, fmt.Errorf("dem: %w", err)
	}
	metrics.S3Requests.WithLabelValues("ok").Inc()
	defer body.Close()
	raw, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("dem: reading %s: %w", name, err)
	}
	return parseTile(bytes.Clone(raw), name, l.f)
}

type S3Source struct {
	cachedSource
}

func NewS3Source(client *s3.Client, prefix string, maxTiles int) *S3Source {
	l := s3Loader{client: client, prefix: prefix, f: hgt, timeout: 60 * time.Second}
	return &S3Source{cachedSource{newTileCache("dem", maxTiles, l.load)}}
}
