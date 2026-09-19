package itm

import "math"

func terrainRoughness(d__meter, delta_h__meter float64) float64 {
	// [ERL 79-ITS 67, Eqn 3], with distance in meters instead of kilometers
	return delta_h__meter * (1.0 - 0.8*math.Exp(-d__meter/50e3))
}
