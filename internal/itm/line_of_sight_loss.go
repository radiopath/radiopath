package itm

import "math"

func lineOfSightLoss(d__meter float64, h_e__meter [2]float64, Z_g complex128, delta_h__meter, M_d, A_d0, d_sML__meter, f__mhz float64) float64 {
	delta_h_d__meter := terrainRoughness(d__meter, delta_h__meter)

	sigma_h_d__meter := sigmaHFunction(delta_h_d__meter)

	wn := f__mhz / 47.7

	// [Algorithm, Eqn 4.46]
	sin_psi := (h_e__meter[0] + h_e__meter[1]) / math.Sqrt(math.Pow(d__meter, 2)+math.Pow(h_e__meter[0]+h_e__meter[1], 2))

	// [Algorithm, Eqn 4.47]
	R_e := (complex(sin_psi, 0) - Z_g) / (complex(sin_psi, 0) + Z_g) * complex(math.Exp(-math.Min(10.0, wn*sigma_h_d__meter*sin_psi)), 0)

	// q = Magnitude of R_e', [Algorithm, Eqn 4.48]
	q := math.Pow(real(R_e), 2) + math.Pow(imag(R_e), 2)
	if q < 0.25 || q < sin_psi {
		R_e = R_e * complex(math.Sqrt(sin_psi/q), 0)
	}

	// phase difference between rays, [Algorithm, Eqn 4.49]
	delta_phi := wn * 2.0 * h_e__meter[0] * h_e__meter[1] / d__meter

	// [Algorithm, Eqn 4.50]
	if delta_phi > pi/2.0 {
		delta_phi = pi - math.Pow(pi/2.0, 2)/delta_phi
	}

	rr := complex(math.Cos(delta_phi), -math.Sin(delta_phi)) + R_e
	A_t__db := -10 * math.Log10(math.Pow(real(rr), 2)+math.Pow(imag(rr), 2))

	A_d__db := M_d*d__meter + A_d0

	w := 1 / (1 + f__mhz*delta_h__meter/math.Max(10e3, d_sML__meter))

	A_los__db := w*A_t__db + (1-w)*A_d__db

	return A_los__db
}
