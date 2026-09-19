package itm

import "math"

func fFunction(td float64) float64 {
	a := [...]float64{133.4, 104.6, 71.8}
	b := [...]float64{0.332e-3, 0.212e-3, 0.157e-3}
	c := [...]float64{-10, -2.5, 5}

	var i int
	if td <= 10e3 {
		i = 0
	} else if td <= 70e3 {
		i = 1
	} else {
		i = 2
	}

	return a[i] + b[i]*td + c[i]*math.Log10(td)
}

func troposcatterLoss(d__meter float64, theta_hzn, d_hzn__meter, h_e__meter [2]float64, a_e__meter, N_s, f__mhz, theta_los float64, h0 *float64) float64 {
	var H_0 float64

	wn := f__mhz / 47.7

	if *h0 > 15.0 {
		H_0 = *h0
	} else {
		ad := d_hzn__meter[0] - d_hzn__meter[1]
		rr := h_e__meter[1] / h_e__meter[0]

		if ad < 0.0 {
			ad = -ad
			rr = 1.0 / rr
		}

		theta := theta_hzn[0] + theta_hzn[1] + d__meter/a_e__meter

		// [TN101, Eqn 9.4a]
		r_1 := 2.0 * wn * theta * h_e__meter[0]
		r_2 := 2.0 * wn * theta * h_e__meter[1]

		if r_1 < 0.2 && r_2 < 0.2 {
			return 1001
		}

		s := (d__meter - ad) / (d__meter + ad)

		q := math.Min(math.Max(0.1, rr/s), 10.0) // TN101, Eqn 9.5
		s = math.Max(0.1, s)                     // TN101, Eqn 9.5

		h_0__meter := (d__meter - ad) * (d__meter + ad) * theta * 0.25 / d__meter // height of cross-over, [Algorithm, 4.66] [TN101v1, 9.3b]

		const Z_0__meter = 1.7556e3
		const Z_1__meter = 8.0e3
		eta_s := (h_0__meter / Z_0__meter) * (1.0 + (0.031-N_s*2.32e-3+math.Pow(N_s, 2)*5.67e-6)*math.Exp(-math.Pow(math.Min(1.7, h_0__meter/Z_1__meter), 6))) // Scattering efficiency factor, eta_s [TN101 Eqn 9.3a]

		H_00 := (h0Function(r_1, eta_s) + h0Function(r_2, eta_s)) / 2 // First term in TN101v1, Eqn 9.5
		Delta_H_0 := math.Min(H_00, 6.0*(0.6-math.Log10(math.Max(eta_s, 1.0)))*math.Log10(s)*math.Log10(q))

		H_0 = H_00 + Delta_H_0   // TN101, Eqn 9.5
		H_0 = math.Max(H_0, 0.0) // "If Delta_H_0 would make H_0 negative, use H_0 = 0" [TN101v1, p9.4]

		if eta_s < 1.0 {
			H_0 = eta_s*H_0 + (1.0-eta_s)*10*math.Log10(math.Pow((1.0+math.Sqrt2/r_1)*(1.0+math.Sqrt2/r_2), 2)*(r_1+r_2)/(r_1+r_2+2*math.Sqrt2))
		}

		if H_0 > 15.0 && *h0 >= 0.0 {
			H_0 = *h0
		}
	}

	*h0 = H_0
	th := d__meter/a_e__meter - theta_los

	const D_0__meter = 40e3
	const H__meter = 47.7
	return fFunction(th*d__meter) + 10*math.Log10(wn*H__meter*math.Pow(th, 4)) - 0.1*(N_s-301.0)*math.Exp(-th*d__meter/D_0__meter) + H_0
}
