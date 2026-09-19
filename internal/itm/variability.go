package itm

import "math"

// TN101v2 Eqn III.69 & III.70
func curve(c1, c2, x1, x2, x3, d_e__meter float64) float64 {
	return (c1 + c2/(1.0+math.Pow((d_e__meter-x2)/x3, 2))) * (math.Pow(d_e__meter/x1, 2)) / (1.0 + (math.Pow(d_e__meter/x1, 2)))
}

func variability(time, location, situation float64, h_e__meter [2]float64, delta_h__meter, f__mhz, d__meter, A_ref__db float64,
	climate, mdvar int, warnings *Warning) float64 {
	// -> approximate to TN101v2 Eqn III.69 & III.70
	all_year := [5][7]float64{
		{-9.67, -0.62, 1.26, -9.21, -0.62, -0.39, 3.15},
		{12.7, 9.19, 15.5, 9.05, 9.19, 2.86, 857.9},
		{144.9e3, 228.9e3, 262.6e3, 84.1e3, 228.9e3, 141.7e3, 2222.e3},
		{190.3e3, 205.2e3, 185.2e3, 101.1e3, 205.2e3, 315.9e3, 164.8e3},
		{133.8e3, 143.6e3, 99.8e3, 98.6e3, 143.6e3, 167.4e3, 116.3e3},
	}

	bsm1 := [...]float64{2.13, 2.66, 6.11, 1.98, 2.68, 6.86, 8.51}
	bsm2 := [...]float64{159.5, 7.67, 6.65, 13.11, 7.16, 10.38, 169.8}
	xsm1 := [...]float64{762.2e3, 100.4e3, 138.2e3, 139.1e3, 93.7e3, 187.8e3, 609.8e3}
	xsm2 := [...]float64{123.6e3, 172.5e3, 242.2e3, 132.7e3, 186.8e3, 169.6e3, 119.9e3}
	xsm3 := [...]float64{94.5e3, 136.4e3, 178.6e3, 193.5e3, 133.5e3, 108.9e3, 106.6e3}

	bsp1 := [...]float64{2.11, 6.87, 10.08, 3.68, 4.75, 8.58, 8.43}
	bsp2 := [...]float64{102.3, 15.53, 9.60, 159.3, 8.12, 13.97, 8.19}
	xsp1 := [...]float64{636.9e3, 138.7e3, 165.3e3, 464.4e3, 93.2e3, 216.0e3, 136.2e3}
	xsp2 := [...]float64{134.8e3, 143.7e3, 225.7e3, 93.1e3, 135.9e3, 152.0e3, 188.5e3}
	xsp3 := [...]float64{95.6e3, 98.6e3, 129.7e3, 94.2e3, 113.4e3, 122.7e3, 122.9e3}

	C_D := [...]float64{1.224, 0.801, 1.380, 1.000, 1.224, 1.518, 1.518}
	z_D := [...]float64{1.282, 2.161, 1.282, 20.0, 1.282, 1.282, 1.282}

	bfm1 := [...]float64{1.0, 1.0, 1.0, 1.0, 0.92, 1.0, 1.0}
	bfm2 := [...]float64{0.0, 0.0, 0.0, 0.0, 0.25, 0.0, 0.0}
	bfm3 := [...]float64{0.0, 0.0, 0.0, 0.0, 1.77, 0.0, 0.0}

	bfp1 := [...]float64{1.0, 0.93, 1.0, 0.93, 0.93, 1.0, 1.0}
	bfp2 := [...]float64{0.0, 0.31, 0.0, 0.19, 0.31, 0.0, 0.0}
	bfp3 := [...]float64{0.0, 2.00, 0.0, 1.79, 2.00, 0.0, 0.0}

	z_T := inverseComplementaryCumulativeDistributionFunction(time / 100)
	z_L := inverseComplementaryCumulativeDistributionFunction(location / 100)
	z_S := inverseComplementaryCumulativeDistributionFunction(situation / 100)

	climate_idx := climate - 1

	wn := f__mhz / 47.7

	d_ex__meter := math.Sqrt(2*a_9000__meter*h_e__meter[0]) + math.Sqrt(2*a_9000__meter*h_e__meter[1]) + math.Pow(575.7e12/wn, third) // [Algorithm, Eqn 5.3]

	var d_e__meter float64
	if d__meter < d_ex__meter {
		d_e__meter = 130e3 * d__meter / d_ex__meter
	} else {
		d_e__meter = 130e3 + d__meter - d_ex__meter
	}

	mdvar_internal := mdvar
	plus20 := mdvar_internal >= 20
	if plus20 {
		mdvar_internal -= 20
	}

	var sigma_S float64
	if plus20 {
		sigma_S = 0.0
	} else {
		D__meter := 100e3
		sigma_S = 5.0 + 3.0*math.Exp(-d_e__meter/D__meter) // [Algorithm, Eqn 5.10]
	}

	plus10 := mdvar_internal >= 10
	if plus10 {
		mdvar_internal -= 10
	}

	V_med__db := curve(all_year[0][climate_idx], all_year[1][climate_idx], all_year[2][climate_idx], all_year[3][climate_idx], all_year[4][climate_idx], d_e__meter)

	if mdvar_internal == SingleMessageMode {
		z_T = z_S
		z_L = z_S
	} else if mdvar_internal == AccidentalMode {
		z_L = z_S
	} else if mdvar_internal == MobileMode {
		z_L = z_T
	}

	if math.Abs(z_T) > 3.10 || math.Abs(z_L) > 3.10 || math.Abs(z_S) > 3.10 {
		*warnings |= WarnExtremeVariabilities
	}

	var sigma_L float64
	if plus10 {
		sigma_L = 0.0
	} else {
		delta_h_d__meter := terrainRoughness(d__meter, delta_h__meter)

		sigma_L = 10.0 * wn * delta_h_d__meter / (wn*delta_h_d__meter + 13.0) // Context of [Algorithm, Eqn 5.9]
	}
	Y_L := sigma_L * z_L

	q := math.Log(0.133 * wn)
	g_minus := bfm1[climate_idx] + bfm2[climate_idx]/(math.Pow(bfm3[climate_idx]*q, 2)+1.0)
	g_plus := bfp1[climate_idx] + bfp2[climate_idx]/(math.Pow(bfp3[climate_idx]*q, 2)+1.0)

	sigma_T_minus := curve(bsm1[climate_idx], bsm2[climate_idx], xsm1[climate_idx], xsm2[climate_idx], xsm3[climate_idx], d_e__meter) * g_minus
	sigma_T_plus := curve(bsp1[climate_idx], bsp2[climate_idx], xsp1[climate_idx], xsp2[climate_idx], xsp3[climate_idx], d_e__meter) * g_plus

	sigma_TD := C_D[climate_idx] * sigma_T_plus
	tgtd := (sigma_T_plus - sigma_TD) * z_D[climate_idx]

	var sigma_T float64
	if z_T < 0.0 {
		sigma_T = sigma_T_minus
	} else if z_T <= z_D[climate_idx] {
		sigma_T = sigma_T_plus
	} else {
		sigma_T = sigma_TD + tgtd/z_T
	}
	Y_T := sigma_T * z_T

	Y_S_temp := math.Pow(sigma_S, 2) + math.Pow(Y_T, 2)/(7.8+math.Pow(z_S, 2)) + math.Pow(Y_L, 2)/(24.0+math.Pow(z_S, 2)) // Part of [Algorithm, Eqn 5.11]

	var Y_R, Y_S float64
	if mdvar_internal == SingleMessageMode {
		Y_R = 0.0
		Y_S = math.Sqrt(math.Pow(sigma_T, 2)+math.Pow(sigma_L, 2)+Y_S_temp) * z_S
	} else if mdvar_internal == AccidentalMode {
		Y_R = Y_T
		Y_S = math.Sqrt(math.Pow(sigma_L, 2)+Y_S_temp) * z_S
	} else if mdvar_internal == MobileMode {
		Y_R = math.Sqrt(math.Pow(sigma_T, 2)+math.Pow(sigma_L, 2)) * z_T
		Y_S = math.Sqrt(Y_S_temp) * z_S
	} else {
		Y_R = Y_T + Y_L
		Y_S = math.Sqrt(Y_S_temp) * z_S
	}

	result := A_ref__db - V_med__db - Y_R - Y_S

	// [Algorithm, Eqn 52]
	if result < 0.0 {
		result = result * (29.0 - result) / (29.0 - 10.0*result)
	}

	return result
}
