package itm

func findHorizons(pfl []float64, a_e__meter float64, h__meter [2]float64, theta_hzn, d_hzn__meter *[2]float64) {
	np := int(pfl[0])
	xi := pfl[1]

	d__meter := pfl[0] * pfl[1]

	z_tx__meter := pfl[2] + h__meter[0]
	z_rx__meter := pfl[np+2] + h__meter[1]

	// [TN101, Eq 6.15]
	theta_hzn[0] = (z_rx__meter-z_tx__meter)/d__meter - d__meter/(2*a_e__meter)
	theta_hzn[1] = -(z_rx__meter-z_tx__meter)/d__meter - d__meter/(2*a_e__meter)

	d_hzn__meter[0] = d__meter
	d_hzn__meter[1] = d__meter

	d_tx__meter := 0.0
	d_rx__meter := d__meter

	for i := 1; i < np; i++ {
		d_tx__meter = d_tx__meter + xi
		d_rx__meter = d_rx__meter - xi

		theta_tx := (pfl[i+2]-z_tx__meter)/d_tx__meter - d_tx__meter/(2*a_e__meter)
		theta_rx := -(z_rx__meter-pfl[i+2])/d_rx__meter - d_rx__meter/(2*a_e__meter)

		if theta_tx > theta_hzn[0] {
			theta_hzn[0] = theta_tx
			d_hzn__meter[0] = d_tx__meter
		}

		if theta_rx > theta_hzn[1] {
			theta_hzn[1] = theta_rx
			d_hzn__meter[1] = d_rx__meter
		}
	}
}
