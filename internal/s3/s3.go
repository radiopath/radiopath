package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrNotFound = errors.New("s3: object not found")

type Client struct {
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string

	HTTP *http.Client
	now  func() time.Time
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (c *Client) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	url := strings.TrimSuffix(c.Endpoint, "/") + "/" + c.Bucket + "/" + encodePath(key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if c.AccessKey != "" {
		now := time.Now
		if c.now != nil {
			now = c.now
		}
		sign(req, c.AccessKey, c.SecretKey, c.Region, now().UTC())
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return resp.Body, nil
	case http.StatusNotFound:
		resp.Body.Close()
		return nil, fmt.Errorf("%w: %s", ErrNotFound, key)
	default:
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		resp.Body.Close()
		return nil, fmt.Errorf("s3: GET %s: status %d: %s", key, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
}
