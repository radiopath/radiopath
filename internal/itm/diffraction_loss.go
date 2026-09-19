package itm

import "math"

func diffractionLoss(d__meter float64, d_hzn__meter, h_e__meter [2]float64, Z_g complex128, a_e__meter, delta_h__meter float64,
	h__meter [2]float64, mode int, theta_los, d_sML__meter, f__mhz float64) float64 {
	A_k__db := knifeEdgeDiffraction(d__meter, f__mhz, a_e__meter, theta_los, d_hzn__meter)

	A_se__db := smoothEarthDiffraction(d__meter, f__mhz, a_e__meter, theta_los, d_hzn__meter, h_e__meter, Z_g)

	// Terrain roughness term, using d_sML__meter, per [ERL 79-ITS 67, page 3-13]
	delta_h_dsML__meter := terrainRoughness(d_sML__meter, delta_h__meter)

	sigma_h_d__meter := sigmaHFunction(delta_h_dsML__meter)

	// [ERL 79-ITS 67, Eqn 3.38c]
	A_fo__db := math.Min(15.0, 5*math.Log10(1.0+1e-5*h__meter[0]*h__meter[1]*f__mhz*sigma_h_d__meter))

	delta_h_d__meter := terrainRoughness(d__meter, delta_h__meter)

	q := h__meter[0] * h__meter[1]
	qk := h_e__meter[0]*h_e__meter[1] - q

	// For low antennas with known path parameters, C ~= 10 [ERL 79-ITS 67, page 3-8]
	if mode == mode__P2P {
		q += 10.0
	}

	term1 := math.Sqrt(1.0 + qk/q) // square root term in [ERL 79-ITS 67, Eqn 3.23]

	d_ML__meter := d_hzn__meter[0] + d_hzn__meter[1]
	q = (term1 + (-theta_los*a_e__meter+d_ML__meter)/d__meter) * math.Min(delta_h_d__meter*f__mhz/47.7, 6283.2)

	// weighting factor [ERL 17-ITS 67, Eqn 3.23]
	w := 25.1 / (25.1 + math.Sqrt(q))

	A_d__db := w*A_se__db + (1.0-w)*A_k__db + A_fo__db

	return A_d__db
}
