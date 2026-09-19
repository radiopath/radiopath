package dem

import (
	"container/list"
	"fmt"
	"math"
	"sync"

	"github.com/radiopath/radiopath/internal/metrics"
)

type tileCache struct {
	load func(name string) (*tile, error)
	kind string
	max  int

	mu      sync.Mutex
	tiles   map[string]*list.Element
	lru     *list.List
	spacing float64
}

type entry struct {
	name string
	tile *tile
}

func newTileCache(kind string, maxTiles int, load func(string) (*tile, error)) *tileCache {
	if maxTiles < 1 {
		maxTiles = 1
	}
	return &tileCache{load: load, kind: kind, max: maxTiles, tiles: make(map[string]*list.Element), lru: list.New(),
		spacing: defaultSpacingM}
}

func (c *tileCache) get(name string) (*tile, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.tiles[name]; ok {
		c.lru.MoveToFront(el)
		metrics.DEMTiles.WithLabelValues(c.kind, "hit").Inc()
		return el.Value.(*entry).tile, nil
	}
	metrics.DEMTiles.WithLabelValues(c.kind, "miss").Inc()
	t, err := c.load(name)
	if err != nil {
		return nil, err
	}
	c.spacing = t.spacingM()
	c.tiles[name] = c.lru.PushFront(&entry{name: name, tile: t})
	for c.lru.Len() > c.max {
		old := c.lru.Back()
		c.lru.Remove(old)
		delete(c.tiles, old.Value.(*entry).name)
	}
	return t, nil
}

func (c *tileCache) resolutionM() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.spacing
}

type cachedSource struct {
	c *tileCache
}

func (s *cachedSource) ResolutionM() float64 { return s.c.resolutionM() }

func (s *cachedSource) Elevation(lat, lon float64) (float64, error) {
	if math.Abs(lat) > 90 || math.Abs(lon) > 180 {
		return 0, fmt.Errorf("dem: invalid position %v,%v", lat, lon)
	}
	t, err := s.c.get(tileName(lat, lon))
	if err != nil {
		return 0, err
	}
	return t.sample(lat, lon)
}
