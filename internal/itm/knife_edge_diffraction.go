package itm

import "math"

func knifeEdgeDiffraction(d__meter, f__mhz, a_e__meter, theta_los float64, d_hzn__meter [2]float64) float64 {
	d_ML__meter := d_hzn__meter[0] + d_hzn__meter[1]
	theta_nlos := d__meter/a_e__meter - theta_los // Angular distance of diffraction region [Algorithm, Eqn 4.12]

	d_nlos__meter := d__meter - d_ML__meter

	// [TN101, Eqn I.7]
	v_1 := 0.0795775 * (f__mhz / 47.7) * math.Pow(theta_nlos, 2) * d_hzn__meter[0] * d_nlos__meter / (d_nlos__meter + d_hzn__meter[0])
	v_2 := 0.0795775 * (f__mhz / 47.7) * math.Pow(theta_nlos, 2) * d_hzn__meter[1] * d_nlos__meter / (d_nlos__meter + d_hzn__meter[1])

	A_k__db := fresnelIntegral(v_1) + fresnelIntegral(v_2) // [TN101, Eqn I.1]

	return A_k__db
}
