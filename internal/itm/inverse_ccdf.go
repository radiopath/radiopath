package itm

import "math"

// Abramowitz & Stegun 26.2.23
func inverseComplementaryCumulativeDistributionFunction(q float64) float64 {
	const (
		C_0 = 2.515516
		C_1 = 0.802853
		C_2 = 0.010328
		D_1 = 1.432788
		D_2 = 0.189269
		D_3 = 0.001308
	)

	x := q
	if q > 0.5 {
		x = 1.0 - x
	}

	T_x := math.Sqrt(-2.0 * math.Log(x))

	zeta_x := ((C_2*T_x+C_1)*T_x + C_0) / (((D_3*T_x+D_2)*T_x+D_1)*T_x + 1.0)

	Q_q := T_x - zeta_x

	if q > 0.5 {
		Q_q = -Q_q
	}

	return Q_q
}
