package s3

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSignExample(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://examplebucket.s3.amazonaws.com/test.txt", nil)
	req.Header.Set("Range", "bytes=0-9")
	now := time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC)
	sign(req, "AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "us-east-1", now)

	want := "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request, " +
		"SignedHeaders=host;range;x-amz-content-sha256;x-amz-date, " +
		"Signature=f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41"
	if got := req.Header.Get("Authorization"); got != want {
		t.Errorf("Authorization:\n got %s\nwant %s", got, want)
	}
	if req.Header.Get("X-Amz-Date") != "20130524T000000Z" || req.Host != "examplebucket.s3.amazonaws.com" {
		t.Errorf("date %q host %q", req.Header.Get("X-Amz-Date"), req.Host)
	}
}

func TestEncodePath(t *testing.T) {
	if got := encodePath("dem/N47E009.hgt"); got != "dem/N47E009.hgt" {
		t.Errorf("plain key encoded as %s", got)
	}
	if got := encodePath("a b/c+d/é"); got != "a%20b/c%2Bd/%C3%A9" {
		t.Errorf("special chars encoded as %s", got)
	}
}

func TestGet(t *testing.T) {
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		switch r.URL.Path {
		case "/tiles/dem/N47E009.hgt":
			w.Write([]byte("data"))
		case "/tiles/dem/missing.hgt":
			w.WriteHeader(404)
		default:
			w.WriteHeader(403)
			w.Write([]byte("<Error>AccessDenied</Error>"))
		}
	}))
	defer srv.Close()

	c := &Client{Endpoint: srv.URL + "/", Bucket: "tiles", Region: "us-east-1", AccessKey: "AK", SecretKey: "SK"}
	body, err := c.Get(context.Background(), "dem/N47E009.hgt")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(body)
	body.Close()
	if string(b) != "data" || gotPath != "/tiles/dem/N47E009.hgt" {
		t.Errorf("body %q path %s", b, gotPath)
	}
	if !strings.HasPrefix(gotAuth, "AWS4-HMAC-SHA256 Credential=AK/") {
		t.Errorf("not signed: %q", gotAuth)
	}

	if _, err := c.Get(context.Background(), "dem/missing.hgt"); !errors.Is(err, ErrNotFound) {
		t.Errorf("404: %v", err)
	}
	if _, err := c.Get(context.Background(), "dem/denied.hgt"); err == nil || !strings.Contains(err.Error(), "403") {
		t.Errorf("403: %v", err)
	}

	anon := &Client{Endpoint: srv.URL, Bucket: "tiles"}
	if body, err := anon.Get(context.Background(), "dem/N47E009.hgt"); err != nil {
		t.Fatal(err)
	} else {
		body.Close()
	}
	if gotAuth != "" {
		t.Errorf("anonymous request was signed: %q", gotAuth)
	}
}
