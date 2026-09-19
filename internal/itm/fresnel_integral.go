package itm

import "math"

func fresnelIntegral(v2 float64) float64 {
	if v2 < 5.76 {
		return 6.02 + 9.11*math.Sqrt(v2) - 1.27*v2 // [TN101v2, Eqn III.24b] and [ERL 79-ITS 67, Eqn 3.27a & 3.27b]
	}
	return 12.953 + 10*math.Log10(v2) // [TN101v2, Eqn III.24c] and [ERL 79-ITS 67, Eqn 3.27a & 3.27b]
}
