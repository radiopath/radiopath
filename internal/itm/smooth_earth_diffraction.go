package itm

import (
	"math"
	"math/cmplx"
)

// Vogler three-radii method
func smoothEarthDiffraction(d__meter, f__mhz, a_e__meter, theta_los float64, d_hzn__meter, h_e__meter [2]float64, Z_g complex128) float64 {
	var a__meter, d__km, K, B_0, x__km, C_0 [3]float64
	var F_x__db [2]float64

	theta_nlos := d__meter/a_e__meter - theta_los // [Algorithm, Eqn 4.12]
	d_ML__meter := d_hzn__meter[0] + d_hzn__meter[1]

	a__meter[0] = (d__meter - d_ML__meter) / (d__meter/a_e__meter - theta_los)
	a__meter[1] = 0.5 * math.Pow(d_hzn__meter[0], 2) / h_e__meter[0] // [Vogler 1964, Eqn 3] rearranged
	a__meter[2] = 0.5 * math.Pow(d_hzn__meter[1], 2) / h_e__meter[1] // [Vogler 1964, Eqn 3] rearranged

	d__km[0] = (a__meter[0] * theta_nlos) / 1000.0
	d__km[1] = d_hzn__meter[0] / 1000.0
	d__km[2] = d_hzn__meter[1] / 1000.0

	for i := 0; i < 3; i++ {
		// C_0 = (4 / 3k) ^ (1 / 3) [Vogler 1964, Eqn 2]
		C_0[i] = math.Pow((4.0/3.0)*a_0__meter/a__meter[i], third)

		// [Vogler 1964, Eqn 6a / 7a]
		K[i] = 0.017778 * C_0[i] * math.Pow(f__mhz, -third) / cmplx.Abs(Z_g)

		// compute B_0 for each radius [Vogler 1964, Fig 4]
		B_0[i] = 1.607 - K[i]
	}

	// compute x__km for each radius [Vogler 1964, Eqn 2]
	x__km[1] = B_0[1] * math.Pow(C_0[1], 2) * math.Pow(f__mhz, third) * d__km[1]
	x__km[2] = B_0[2] * math.Pow(C_0[2], 2) * math.Pow(f__mhz, third) * d__km[2]
	x__km[0] = B_0[0]*math.Pow(C_0[0], 2)*math.Pow(f__mhz, third)*d__km[0] + x__km[1] + x__km[2]

	F_x__db[0] = heightFunction(x__km[1], K[1])
	F_x__db[1] = heightFunction(x__km[2], K[2])

	G_x__db := 0.05751*x__km[0] - 10.0*math.Log10(x__km[0]) // [TN101, Eqn 8.4] & [Vogler 1964, Eqn 13]

	return G_x__db - F_x__db[0] - F_x__db[1] - 20 // [Algorithm, Eqn 4.20] & [Vogler 1964]
}

func heightFunction(x__km, K float64) float64 {
	var w, result float64

	if x__km < 200.0 {
		w = -math.Log(K)

		if K < 1e-5 || x__km*math.Pow(w, 3) > 5495.0 {
			result = -117.0

			if x__km > 1.0 {
				result = 17.372*math.Log(x__km) + result
			}
		} else {
			result = 2.5e-5*math.Pow(x__km, 2)/K - 8.686*w - 15.0
		}
	} else {
		result = 0.05751*x__km - 4.343*math.Log(x__km)

		if x__km < 2000 {
			w = 0.0134 * x__km * math.Exp(-0.005*x__km)
			result = (1.0-w)*result + w*(17.372*math.Log(x__km)-117.0)
		}
	}

	return result
}
