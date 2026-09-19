package dem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type dirLoader struct {
	dir string
	f   format
}

func (l dirLoader) load(name string) (*tile, error) {
	raw, err := os.ReadFile(filepath.Join(l.dir, name+l.f.ext))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrNoTile, name)
	}
	if err != nil {
		return nil, err
	}
	return parseTile(raw, name, l.f)
}

type DirSource struct {
	cachedSource
	dir string
}

func NewDirSource(dir string, maxTiles int) *DirSource {
	return &DirSource{cachedSource{newTileCache("dem", maxTiles, dirLoader{dir, hgt}.load)}, dir}
}

func (s *DirSource) Ready() error {
	_, err := os.ReadDir(s.dir)
	return err
}
