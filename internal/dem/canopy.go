package dem

import (
	"errors"
	"time"

	"github.com/radiopath/radiopath/internal/s3"
)

type Canopy interface {
	HeightM(lat, lon float64) (float64, error)
}

type CanopySource struct {
	c *tileCache
}

func NewDirCanopy(dir string, maxTiles int) *CanopySource {
	return &CanopySource{newTileCache("canopy", maxTiles, treeless(dirLoader{dir, chm}.load))}
}

func NewS3Canopy(client *s3.Client, prefix string, maxTiles int) *CanopySource {
	l := s3Loader{client: client, prefix: prefix, f: chm, timeout: 60 * time.Second}
	return &CanopySource{newTileCache("canopy", maxTiles, treeless(l.load))}
}

func treeless(load func(string) (*tile, error)) func(string) (*tile, error) {
	return func(name string) (*tile, error) {
		t, err := load(name)
		if errors.Is(err, ErrNoTile) {
			return &tile{side: 2, raw: make([]byte, 4), f: chm}, nil
		}
		return t, err
	}
}

func (s *CanopySource) HeightM(lat, lon float64) (float64, error) {
	t, err := s.c.get(tileName(lat, lon))
	if err != nil {
		return 0, err
	}
	return t.sample(lat, lon)
}
