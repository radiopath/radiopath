package itm

import "math"

func sigmaHFunction(delta_h__meter float64) float64 {
	// [ERL 79-ITS 67, Eqn 3.6a]
	return 0.78 * delta_h__meter * math.Exp(-0.5*math.Pow(delta_h__meter, 0.25))
}
