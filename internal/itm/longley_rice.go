package itm

import "math"

func longleyRice(theta_hzn [2]float64, f__mhz float64, Z_g complex128, d_hzn__meter, h_e__meter [2]float64,
	gamma_e, N_s, delta_h__meter float64, h__meter [2]float64, d__meter float64, mode int,
	A_ref__db *float64, warnings *Warning, propmode *Mode) int {
	a_e__meter := 1 / gamma_e

	var d_hzn_s__meter [2]float64
	for i := 0; i < 2; i++ {
		d_hzn_s__meter[i] = math.Sqrt(2.0 * h_e__meter[i] * a_e__meter)
	}

	d_sML__meter := d_hzn_s__meter[0] + d_hzn_s__meter[1]

	d_ML__meter := d_hzn__meter[0] + d_hzn__meter[1]

	theta_los := -math.Max(theta_hzn[0]+theta_hzn[1], -d_ML__meter/a_e__meter)

	if math.Abs(theta_hzn[0]) > 200e-3 {
		*warnings |= WarnTxHorizonAngle
	}
	if math.Abs(theta_hzn[1]) > 200e-3 {
		*warnings |= WarnRxHorizonAngle
	}

	if d_hzn__meter[0] < 0.1*d_hzn_s__meter[0] {
		*warnings |= WarnTxHorizonDistance1
	}
	if d_hzn__meter[1] < 0.1*d_hzn_s__meter[1] {
		*warnings |= WarnRxHorizonDistance1
	}

	if d_hzn__meter[0] > 3.0*d_hzn_s__meter[0] {
		*warnings |= WarnTxHorizonDistance2
	}
	if d_hzn__meter[1] > 3.0*d_hzn_s__meter[1] {
		*warnings |= WarnRxHorizonDistance2
	}

	if N_s < 150 {
		return error__SURFACE_REFRACTIVITY_SMALL
	}
	if N_s > 400 {
		return error__SURFACE_REFRACTIVITY_LARGE
	}
	if N_s < 250 {
		*warnings |= WarnSurfaceRefractivity
	}

	if a_e__meter < 4000000 || a_e__meter > 13333333 {
		return error__EFFECTIVE_EARTH
	}

	if real(Z_g) <= math.Abs(imag(Z_g)) {
		return error__GROUND_IMPEDANCE
	}

	d_3__meter := math.Max(d_sML__meter, d_ML__meter+5.0*math.Pow(math.Pow(a_e__meter, 2)/f__mhz, 1.0/3.0))
	d_4__meter := d_3__meter + 10.0*math.Pow(math.Pow(a_e__meter, 2)/f__mhz, 1.0/3.0)

	A_3__db := diffractionLoss(d_3__meter, d_hzn__meter, h_e__meter, Z_g, a_e__meter, delta_h__meter, h__meter, mode, theta_los, d_sML__meter, f__mhz)
	A_4__db := diffractionLoss(d_4__meter, d_hzn__meter, h_e__meter, Z_g, a_e__meter, delta_h__meter, h__meter, mode, theta_los, d_sML__meter, f__mhz)

	M_d := (A_4__db - A_3__db) / (d_4__meter - d_3__meter)
	A_d0__db := A_3__db - M_d*d_3__meter

	d_min__meter := math.Abs(h_e__meter[0]-h_e__meter[1]) / 200e-3

	if d__meter < d_min__meter {
		*warnings |= WarnPathDistanceTooSmall1
	}
	if d__meter < 1e3 {
		*warnings |= WarnPathDistanceTooSmall2
	}
	if d__meter > 1000e3 {
		*warnings |= WarnPathDistanceTooBig1
	}
	if d__meter > 2000e3 {
		*warnings |= WarnPathDistanceTooBig2
	}

	if d__meter < d_sML__meter {
		A_sML__db := d_sML__meter*M_d + A_d0__db

		// [ERL 79-ITS 67, Eqn 3.16a], in meters instead of km and with MIN() part below
		d_0__meter := 0.04 * f__mhz * h_e__meter[0] * h_e__meter[1]

		var d_1__meter float64
		if A_d0__db >= 0.0 {
			d_0__meter = math.Min(d_0__meter, 0.5*d_ML__meter)      // other part of [ERL 79-ITS 67, Eqn 3.16a]
			d_1__meter = d_0__meter + 0.25*(d_ML__meter-d_0__meter) // [ERL 79-ITS 67, Eqn 3.16d]
		} else {
			d_1__meter = math.Max(-A_d0__db/M_d, 0.25*d_ML__meter)
		}

		A_1__db := lineOfSightLoss(d_1__meter, h_e__meter, Z_g, delta_h__meter, M_d, A_d0__db, d_sML__meter, f__mhz)

		flag := false

		kHat_1__db_per_meter := 0.0
		kHat_2__db_per_meter := 0.0

		if d_0__meter < d_1__meter {
			A_0__db := lineOfSightLoss(d_0__meter, h_e__meter, Z_g, delta_h__meter, M_d, A_d0__db, d_sML__meter, f__mhz)

			q := math.Log(d_sML__meter / d_0__meter)

			// [ERL 79-ITS 67, Eqn 3.20]
			kHat_2__db_per_meter = math.Max(0.0, ((d_sML__meter-d_0__meter)*(A_1__db-A_0__db)-(d_1__meter-d_0__meter)*(A_sML__db-A_0__db))/((d_sML__meter-d_0__meter)*math.Log(d_1__meter/d_0__meter)-(d_1__meter-d_0__meter)*q))

			flag = A_d0__db > 0.0 || kHat_2__db_per_meter > 0.0

			if flag {
				// [ERL 79-ITS 67, Eqn 3.21]
				kHat_1__db_per_meter = (A_sML__db - A_0__db - kHat_2__db_per_meter*q) / (d_sML__meter - d_0__meter)

				if kHat_1__db_per_meter < 0.0 {
					kHat_1__db_per_meter = 0.0
					kHat_2__db_per_meter = fdim(A_sML__db, A_0__db) / q

					if kHat_2__db_per_meter == 0.0 {
						kHat_1__db_per_meter = M_d
					}
				}
			}
		}

		if !flag {
			kHat_1__db_per_meter = fdim(A_sML__db, A_1__db) / (d_sML__meter - d_1__meter)
			kHat_2__db_per_meter = 0.0

			if kHat_1__db_per_meter == 0.0 {
				kHat_1__db_per_meter = M_d
			}
		}

		A_o__db := A_sML__db - kHat_1__db_per_meter*d_sML__meter - kHat_2__db_per_meter*math.Log(d_sML__meter)

		// [ERL 79-ITS 67, Eqn 3.19]
		*A_ref__db = A_o__db + kHat_1__db_per_meter*d__meter + kHat_2__db_per_meter*math.Log(d__meter)
		*propmode = ModeLineOfSight
	} else {
		d_5__meter := d_ML__meter + 200e3
		d_6__meter := d_ML__meter + 400e3

		h0 := -1.0
		A_6__db := troposcatterLoss(d_6__meter, theta_hzn, d_hzn__meter, h_e__meter, a_e__meter, N_s, f__mhz, theta_los, &h0)
		A_5__db := troposcatterLoss(d_5__meter, theta_hzn, d_hzn__meter, h_e__meter, a_e__meter, N_s, f__mhz, theta_los, &h0)

		var M_s, A_s0__db, d_x__meter float64

		if A_5__db < 1000.0 {
			M_s = (A_6__db - A_5__db) / 200e3

			d_x__meter = math.Max(math.Max(d_sML__meter, d_ML__meter+1.088*math.Pow(math.Pow(a_e__meter, 2)/f__mhz, 1.0/3.0)*math.Log(f__mhz)), (A_5__db-A_d0__db-M_s*d_5__meter)/(M_d-M_s))

			A_s0__db = (M_d-M_s)*d_x__meter + A_d0__db
		} else {
			M_s = M_d
			A_s0__db = A_d0__db
			d_x__meter = 10e6
		}

		if d__meter > d_x__meter {
			*A_ref__db = M_s*d__meter + A_s0__db
			*propmode = ModeTroposcatter
		} else {
			*A_ref__db = M_d*d__meter + A_d0__db
			*propmode = ModeDiffraction
		}
	}

	*A_ref__db = math.Max(*A_ref__db, 0.0)

	return success
}
