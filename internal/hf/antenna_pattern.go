package hf

import (
	"fmt"
	"math"
)

// functionPattern builds the 360 x 91 pattern from gain(azimuth offset, elevation), with the main beam at bearing
// (radians), truncated to a whole degree like the rotation of a type 13 file.
func functionPattern(gain func(azimuthOffset, elevation float64) float64, bearing float64) (*antenna, error) {
	ant := new(antenna)
	allocateAntennaMemory(ant, 1, antAzimuths, antElevations)
	ant.name = "PATTERN"
	mbos := int(bearing * r2d)
	for off := 0; off < antAzimuths; off++ {
		row := ant.pattern[0][(mbos+off)%antAzimuths]
		for el := range row {
			g := gain(float64(off), float64(el))
			if math.IsNaN(g) || math.IsInf(g, 0) {
				return nil, fmt.Errorf("hf: antenna pattern gain is not finite at %d degrees, elevation %d", off, el)
			}
			row[el] = g
		}
	}
	return ant, nil
}
