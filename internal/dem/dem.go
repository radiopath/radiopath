package dem

import "errors"

var (
	ErrNoTile = errors.New("dem: no terrain tile (sea, or not in the store yet)")
	ErrVoid   = errors.New("dem: void data at position")
)

type Source interface {
	Elevation(lat, lon float64) (float64, error)
	ResolutionM() float64
}
