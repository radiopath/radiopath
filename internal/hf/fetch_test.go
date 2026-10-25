package hf

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDataSums checks that the embedded list names every file the model reads, with well-formed hashes.
func TestDataSums(t *testing.T) {
	names := map[string]bool{"P1239-3 Decile Factors.txt": true}
	for _, f := range dataFiles() {
		if len(f.sha256) != 64 {
			t.Errorf("%s: hash %q", f.name, f.sha256)
		}
		names[f.name] = true
	}
	want := []string{"P1239-3 Decile Factors.txt", "ionos01.bin", "ionos12.bin", "COEFF01W.txt", "COEFF12W.txt"}
	for _, n := range want {
		if !names[n] {
			t.Errorf("%s is not in data.sha256", n)
		}
	}
	if len(dataFiles()) != 25 {
		t.Errorf("%d files, want 25", len(dataFiles()))
	}
}

// TestFetchFiles downloads files, rejects a wrong checksum and a missing file, and leaves no partial file behind.
func TestFetchFiles(t *testing.T) {
	sum := func(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
	content := map[string]string{"a.bin": "alpha", "b b.txt": "beta"}
	get := func(_ context.Context, name string) (io.ReadCloser, error) {
		s, ok := content[name]
		if !ok {
			return nil, errors.New("gone")
		}
		return io.NopCloser(strings.NewReader(s)), nil
	}

	dir := t.TempDir()
	files := []dataFile{{"a.bin", sum("alpha")}, {"b b.txt", sum("beta")}}
	if err := fetchFiles(context.Background(), dir, files, get); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "b b.txt")); err != nil || string(b) != "beta" {
		t.Errorf("b b.txt: %q, %v", b, err)
	}

	dir = t.TempDir()
	err := fetchFiles(context.Background(), dir, []dataFile{{"a.bin", sum("other")}}, get)
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Errorf("wrong checksum: %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("left %d files after a mismatch", len(entries))
	}

	if err := fetchFiles(context.Background(), t.TempDir(), []dataFile{{"c.bin", sum("x")}}, get); err == nil || !strings.Contains(err.Error(), "c.bin") {
		t.Errorf("missing file: %v", err)
	}
}
