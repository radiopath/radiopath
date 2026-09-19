package itm

import (
	"math"
	"math/cmplx"
)

func initializePointToPoint(f__mhz, h_sys__meter, N_0 float64, pol int, epsilon, sigma float64) (Z_g complex128, gamma_e, N_s float64) {
	const gamma_a = 157e-9

	if h_sys__meter == 0.0 {
		N_s = N_0
	} else {
		N_s = N_0 * math.Exp(-h_sys__meter/9460.0) // [TN101, Eq 4.3]
	}

	gamma_e = gamma_a * (1.0 - 0.04665*math.Exp(N_s/179.3)) // [TN101, Eq 4.4], reworked

	ep_r := complex(epsilon, 18000*sigma/f__mhz)

	Z_g = cmplx.Sqrt(ep_r - 1.0)

	if pol == int(Vertical) {
		Z_g = Z_g / ep_r
	}
	return Z_g, gamma_e, N_s
}
