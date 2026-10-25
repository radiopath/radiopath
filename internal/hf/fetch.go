package hf

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// dataSums lists the files of the ITU data set with their SHA-256, one "hash  name" per line; scripts/fetch-hf-data.sh
// reads the same file.
//
//go:embed data.sha256
var dataSums string

// dataFile is one file of the data set.
type dataFile struct{ name, sha256 string }

// dataFiles parses the embedded list.
func dataFiles() []dataFile {
	var files []dataFile
	for _, line := range strings.Split(strings.TrimSpace(dataSums), "\n") {
		if hash, name, ok := strings.Cut(line, "  "); ok {
			files = append(files, dataFile{name, hash})
		}
	}
	return files
}

// Fetch downloads the data set into dir, for example from an S3 bucket. get opens one file by name; every file must
// match its pinned SHA-256, so a wrong or tampered copy never reaches the model. Errors of get are wrapped.
func Fetch(ctx context.Context, dir string, get func(ctx context.Context, name string) (io.ReadCloser, error)) error {
	return fetchFiles(ctx, dir, dataFiles(), get)
}

// fetchFiles downloads files into dir and verifies them.
func fetchFiles(ctx context.Context, dir string, files []dataFile, get func(ctx context.Context, name string) (io.ReadCloser, error)) error {
	for _, f := range files {
		if err := fetchFile(ctx, dir, f, get); err != nil {
			return fmt.Errorf("hf: %s: %w", f.name, err)
		}
	}
	return nil
}

// fetchFile downloads one file next to its final name and renames it once the checksum matches.
func fetchFile(ctx context.Context, dir string, f dataFile, get func(ctx context.Context, name string) (io.ReadCloser, error)) error {
	body, err := get(ctx, f.name)
	if err != nil {
		return err
	}
	defer body.Close()
	path := filepath.Join(dir, f.name)
	out, err := os.Create(path + ".part")
	if err != nil {
		return err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(out, h), body)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil && hex.EncodeToString(h.Sum(nil)) != f.sha256 {
		err = fmt.Errorf("checksum mismatch")
	}
	if err != nil {
		os.Remove(path + ".part")
		return err
	}
	return os.Rename(path+".part", path)
}
