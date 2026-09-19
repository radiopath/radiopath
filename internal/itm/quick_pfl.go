package itm

import "math"

func quickPfl(pfl []float64, gamma_e float64, h__meter [2]float64, theta_hzn, d_hzn__meter, h_e__meter *[2]float64,
	delta_h__meter, d__meter *float64, takeoff *[2]float64) {
	var fit_tx, fit_rx, q float64

	*d__meter = pfl[0] * pfl[1]

	np := int(pfl[0])

	a_e__meter := 1 / gamma_e

	findHorizons(pfl, a_e__meter, h__meter, theta_hzn, d_hzn__meter)
	*takeoff = *theta_hzn

	d_start__meter := math.Min(15.0*h__meter[0], 0.1*d_hzn__meter[0])
	d_end__meter := *d__meter - math.Min(15.0*h__meter[1], 0.1*d_hzn__meter[1])

	*delta_h__meter = computeDeltaH(pfl, d_start__meter, d_end__meter)

	if d_hzn__meter[0]+d_hzn__meter[1] > 1.5**d__meter {

		linearLeastSquaresFit(pfl, d_start__meter, d_end__meter, &fit_tx, &fit_rx)

		h_e__meter[0] = h__meter[0] + fdim(pfl[2], fit_tx)
		h_e__meter[1] = h__meter[1] + fdim(pfl[np+2], fit_rx)

		for i := 0; i < 2; i++ {
			d_hzn__meter[i] = math.Sqrt(2.0*h_e__meter[i]*a_e__meter) * math.Exp(-0.07*math.Sqrt(*delta_h__meter/math.Max(h_e__meter[i], 5.0)))
		}

		combined_horizons__meter := d_hzn__meter[0] + d_hzn__meter[1]
		if combined_horizons__meter <= *d__meter {
			q = math.Pow(*d__meter/combined_horizons__meter, 2)

			for i := 0; i < 2; i++ {
				h_e__meter[i] = h_e__meter[i] * q
				d_hzn__meter[i] = math.Sqrt(2.0*h_e__meter[i]*a_e__meter) * math.Exp(-0.07*math.Sqrt(*delta_h__meter/math.Max(h_e__meter[i], 5.0)))
			}
		}

		for i := 0; i < 2; i++ {
			q = math.Sqrt(2.0 * h_e__meter[i] * a_e__meter)
			theta_hzn[i] = (0.65**delta_h__meter*(q/d_hzn__meter[i]-1.0) - 2.0*h_e__meter[i]) / q
		}
	} else {
		dummy := 0.0

		linearLeastSquaresFit(pfl, d_start__meter, 0.9*d_hzn__meter[0], &fit_tx, &dummy)
		h_e__meter[0] = h__meter[0] + fdim(pfl[2], fit_tx)

		linearLeastSquaresFit(pfl, *d__meter-0.9*d_hzn__meter[1], d_end__meter, &dummy, &fit_rx)
		h_e__meter[1] = h__meter[1] + fdim(pfl[np+2], fit_rx)
	}
}
