package hf

import "math"

const (
	noILLong = -0.17 // "Not otherwise included" loss of the long model (dB)

	maxCP = 28 // 26 penetration points of 90 km and 2 control points of Table 1a
	tdM2  = 26 // index of the control point at T + dM/2
	rdM2  = 27 // index of the control point at R - dM/2

	noTime = 99 // indicator of "no time" in findfL()

	south = 1 // winterAnomaly() hemisphere index
	north = 0
)

// medianSkywaveFieldStrengthLong determines the field strength of paths of
// 7000 km and more (P.533 section 5.3 "Paths longer than 7000 km"). For paths
// above 9000 km it is the only method used. It is patterned after FTZ() of
// REC533.
func medianSkywaveFieldStrengthLong(p *pathData) {
	var n, i, j int
	var hour int

	var nL int
	var deltaL, dL float64

	var nM int
	var deltaM, dM float64

	var hr, pt, i90 float64

	var psi, phi, dh90 float64

	var fracd, f, etl, d float64

	var cp [maxCP][24]controlPt

	elevation := 2.0 * pi

	// Only for paths of 7000 km and more; between 7000 and 9000 km the field
	// strength is interpolated with the short method.
	if p.distance >= 7000.0 {
		// The reflection height is 300 km for both fM and fL.
		hr = 300.0

		// fL geometry: equal hops of 3000 km or less for the penetration
		// points.
		for n = 0; p.distance/(float64(n)+1.0) > 3000.0; n++ {
		}

		nL = n

		dL = p.distance / (float64(nL) + 1.0)

		deltaL = elevationAngle(dL, hr)

		// fM geometry: equal hops of 4000 km or less for the control points.
		// A hop is added when the elevation angle is below the minimum.
		for n = 0; p.distance/(float64(n)+1.0) > 4000.0; n++ {
		}

		nM = n

		dM = p.distance / (float64(nM) + 1.0)

		deltaM = elevationAngle(dM, hr)

		if deltaM < minEleAngleL*d2r {
			nM++

			dM = p.distance / (float64(nM) + 1.0)
			deltaM = elevationAngle(dM, hr)
		}

		// Control and penetration point initialization for the reference
		// frequencies. Both fM and fL need 24 hours of data: at the control
		// points of Table 1a for fM and at the 90 km penetration points for
		// fL (up to 13 hops with two penetrations each).

		// The 90 km penetration points.
		i90 = incidenceAngle(deltaL, 90.0)

		// The 90-km height half-hop distance.
		phi = (pi/2.0 - deltaL - i90)

		dh90 = r0 * phi

		// The path is used to determine the data at the control points.
		hour = p.hour

		for j = 0; j < 24; j++ { // hours
			p.hour = j

			for i = 0; i <= nL; i++ { // 90-km penetration points
				zeroCP(&cp[2*i][j])
				zeroCP(&cp[(2*i)+1][j])

				// Two control points per hop: first the end nearest the tx.
				fracd = (float64(i)*dL + dh90) / p.distance
				greatCirclePoint(p.L_tx, p.L_rx, &cp[2*i][j], p.distance, fracd)
				calculateCPParameters(p, &cp[2*i][j])

				cp[2*i][j].hr = 90.0

				// Next the end nearest the receiver for this hop.
				fracd = (float64(i+1)*dL - dh90) / p.distance
				greatCirclePoint(p.L_tx, p.L_rx, &cp[(2*i)+1][j], p.distance, fracd)
				calculateCPParameters(p, &cp[(2*i)+1][j])

				cp[(2*i)+1][j].hr = 90.0
			}

			// The control points T + d0/2 and R - d0/2 of Table 1a are the
			// last two in the array.
			fracd = (1.0 / (2.0 * float64(nM+1)))
			greatCirclePoint(p.L_tx, p.L_rx, &cp[tdM2][j], p.distance, fracd)
			fracd = (1.0 - (1.0 / (2.0 * float64(nM+1))))
			greatCirclePoint(p.L_tx, p.L_rx, &cp[rdM2][j], p.distance, fracd)

			calculateCPParameters(p, &cp[tdM2][j])
			calculateCPParameters(p, &cp[rdM2][j])

			cp[tdM2][j].x = 0.0
			cp[tdM2][j].foE = 0.0
			cp[tdM2][j].hr = 300.0 // fixed at 300 km for this calculation

			cp[rdM2][j].x = 0.0
			cp[rdM2][j].foE = 0.0
			cp[rdM2][j].hr = 300.0
		}

		// Restore the hour.
		p.hour = hour

		// The data of the control points is now available.

		// The virtual slant range (19).
		psi = dM / (2.0 * r0)
		p.ptick = math.Abs(2.0*r0*(math.Sin(psi)/math.Cos(deltaM+psi))) * (float64(nM) + 1.0)

		// The free space field strength.
		p.E0 = 139.6 - 20.0*math.Log10(p.ptick)

		p.Gtl = antennaGain08(p, &p.A_tx, txToRx, &elevation)

		// The focusing on long distance gain, limited to 15 dB.
		d = p.distance

		p.Gap = cMin(10.0*math.Log10(d/(r0*math.Abs(math.Sin(d/r0)))), 15.0)

		p.Ly = noILLong

		// The transmitter power dB (1 kW).
		pt = p.txpower

		// The mean gyrofrequency.
		p.fH = (cp[tdM2][p.hour].fH[hr300km] + cp[rdM2][p.hour].fH[hr300km]) / 2.0

		// The MUF fM.
		findMUFsandfM(p, &cp, nM, dM)

		// The lower frequency.
		findfL(p, &cp, nL, dL, p.ptick, p.fH, i90)

		f = p.frequency

		// Etl in sections.
		etl = (math.Pow((p.fL+p.fH), 2) / math.Pow((f+p.fH), 2)) + (math.Pow((f+p.fH), 2) / math.Pow((p.fM+p.fH), 2))
		etl = etl * (math.Pow((p.fM+p.fH), 2) / ((math.Pow((p.fM + p.fH), 2)) + math.Pow((p.fL+p.fH), 2)))

		p.F = 1.0 - etl

		etl = p.E0 * (1.0 - etl)
		etl = etl - 30.0 + pt + p.Gtl + p.Gap - p.Ly
		p.El = etl

		// For paths longer than 9000 km this is the only method used, so set
		// the path parameters appropriately.
		if p.distance > 9000.0 {
			// The elevation angle of the fM calculation represents the long path.
			p.ele = deltaM

			// Copy the control points to the path.
			copyCP(cp[rdM2][p.hour], &p.CP[rd02])
			copyCP(cp[tdM2][p.hour], &p.CP[td02])

			// Copy the two extreme penetration points to control points.
			copyCP(cp[0][p.hour], &p.CP[t1k])
			copyCP(cp[2*nL][p.hour], &p.CP[r1k])

			p.dmax = 4000.0
		}
	}
}

// findMUFsandfM finds the upper reference frequency fM from 24 hours of
// calculated MUFs and determines the basic and operational MUFs and their
// deciles for the long path.
func findMUFsandfM(p *pathData, cp *[maxCP][24]controlPt, hops int, dM float64) {
	_ = hops // unused, kept for the signature of the C code

	var f4, fz [2]float64
	var fD, deltal, deltau float64
	var decile int
	var smallerCP int
	var fBMmin [2]float64

	// Values used in the determination of K.
	w := [2]float64{0.1, 0.2}
	x := [2]float64{1.2, 0.2}
	y := [2]float64{0.6, 0.4}
	var a, ew, iw, ix, iy float64
	var fBM [2][24]float64 // F2(D)MUF at the control points for 24 hours

	var noon [2]int

	// The distance reduction factor fD from NBS Report 7619, D. Lucas, 1963,
	// with the coefficients changed for hop distances in km.
	fD = ((((((-2.40074637494790e-24*dM+
		25.8520201885984e-21)*dM+
		-92.4986988833091e-18)*dM+
		102.342990689362e-15)*dM+
		22.0776941764705e-12)*dM+
		87.4376851991085e-9)*dM +
		29.1996868566837e-6) * dM

	// The index of local noon (UTC) at T + d0/2 and R - d0/2. The control
	// points have the same longitude for all hours so any hour will do; one is
	// subtracted because the value is used as an index.
	noon[0] = int(12.0-cp[tdM2][1].L.lng/(15.0*d2r)) - 1
	noon[1] = int(12.0-cp[rdM2][1].L.lng/(15.0*d2r)) - 1

	// Roll over the time.
	noon[0] = (noon[0] + 24) % 24
	noon[1] = (noon[1] + 24) % 24

	// The minimum MUF in 24 hours.
	fBMmin[0] = 100.0
	fBMmin[1] = 100.0
	for t := 0; t < 24; t++ {
		// F2(4000)MUF
		f4[0] = 1.1 * cp[tdM2][t].foF2 * cp[tdM2][t].M3kF2
		// F2(ZERO)MUF
		fz[0] = cp[tdM2][t].foF2 + 0.5*cp[tdM2][t].fH[hr300km]

		// The basic MUF
		fBM[0][t] = fz[0] + (f4[0]-fz[0])*fD
		fBMmin[0] = cMin(fBM[0][t], fBMmin[0])

		f4[1] = 1.1 * cp[rdM2][t].foF2 * cp[rdM2][t].M3kF2
		fz[1] = cp[rdM2][t].foF2 + 0.5*cp[rdM2][t].fH[hr300km]

		fBM[1][t] = fz[1] + (f4[1]-fz[1])*fD
		fBMmin[1] = cMin(fBM[1][t], fBMmin[1])
	}

	// The forward azimuth at the mid-point interpolates W, X and Y to
	// calculate K.
	a = bearing(p.CP[mp].L, p.L_rx, shortPath)

	if a > pi {
		a -= pi
	}
	if a >= pi/2.0 {
		a -= pi / 2.0
	} else {
		a = pi/2.0 - a
	}

	ew = a / (pi / 2.0)
	iw = w[0]*(1.0-ew) + w[1]*ew
	iy = y[0]*(1.0-ew) + y[1]*ew
	ix = x[0]*(1.0-ew) + x[1]*ew

	// fBM,min has been determined for both control points: now K.
	for n := 0; n < 2; n++ {
		p.K[n] = 1.2 + iw*(fBM[n][p.hour]/fBM[n][noon[n]]) + ix*(math.Pow((fBM[n][noon[n]]/fBM[n][p.hour]), 1.0/3.0)-1.0) + iy*math.Pow((fBMmin[n]/fBM[n][noon[n]]), 2)
	}

	p.fM = cMin(p.K[0]*fBM[0][p.hour], p.K[1]*fBM[1][p.hour])

	// Set the other MUF parameters, which MUFBasic() determines for shorter
	// paths.
	if p.distance > 9000 {
		smallerCP = 99
		if fBM[0][p.hour] < fBM[1][p.hour] {
			smallerCP = tdM2
			// The basic MUF of the path.
			p.BMUF = fBM[0][p.hour]
		} else if fBM[0][p.hour] >= fBM[1][p.hour] {
			smallerCP = rdM2
			p.BMUF = fBM[1][p.hour]
		}

		// The MUF deciles.
		decile = dl
		deltal = findfoF2var(p, cp[smallerCP][p.hour].ltime, cp[smallerCP][p.hour].L.lat, decile)

		decile = du
		deltau = findfoF2var(p, cp[smallerCP][p.hour].ltime, cp[smallerCP][p.hour].L.lat, decile)

		// The decile MUFs.
		p.MUF50 = p.BMUF
		p.MUF10 = p.MUF50 * deltau
		p.MUF90 = p.MUF50 * deltal

		// The operational MUF and its deciles.
		p.OPMUF = p.fM
		p.OPMUF10 = p.OPMUF * deltau
		p.OPMUF90 = p.OPMUF * deltal
	}
}

// findfL determines the lower reference frequency fL from 24 hours of solar
// zenith angles at the penetration points.
func findfL(p *pathData, cp *[maxCP][24]controlPt, hops int, dh float64, ptick, fH, i90 float64) {
	_ = dh // unused, kept for the signature of the C code

	var sumCosChi [24]float64
	var fL [24]float64
	var chi, dt float64
	var tr, prev, now int

	month := p.month

	for t := 0; t < 24; t++ {
		sumCosChi[t] = 0
		for i := 0; i < 2*(hops+1); i++ {
			chi = cp[i][t].Sun.sza
			if chi > 0.0 && chi < pi/2.0 {
				// Sum over the control points.
				sumCosChi[t] += math.Sqrt(math.Cos(chi))
			}
		}
	}

	// The winter-anomaly factor at the path mid-point.
	aw := winterAnomaly(p.CP[mp].L.lat, month)

	fLN := math.Sqrt(p.distance / 3000.0)

	// All 24 values of the LUF are calculated first so that the transition
	// from day to night, not night to day, can be found. The SSN may exceed
	// MAXSSN here.
	for i := 0; i < 24; i++ {
		fL[i] = cMax((5.3*math.Sqrt(((1.0+0.009*float64(p.SSN))*sumCosChi[i])/(math.Cos(i90)*math.Log(9.5e6/ptick)))-fH)*(aw+1.0), fLN)
	}

	tr = noTime // the reference time indicator

	// The first time that fL[i] <= 2.0*fLN: decay from day-LUF to night-LUF.
	for now = 0; now < 24; now++ {
		prev = ((now - 1) + 24) % 24

		if tr == noTime { // only one reference time in 24 hours
			if fL[prev] >= 2.0*fLN && fL[now] <= 2.0*fLN { // the LUF decreases into night
				tr = now
				dt = (2.0*fLN - fL[tr]) / (fL[prev] - fL[tr])
				fL[tr] = 0.7945 * fL[prev] * (dt*(1.0-0.7945) + 0.7945)
				if fL[now] < fL[tr] {
					fL[now] = fL[tr]
				}
			}
		}
	}

	if tr != noTime {
		for i := 1; i < 4; i++ {
			now = ((tr + i) + 24) % 24

			prev = ((now - 1) + 24) % 24

			fL[now] = cMax(fL[prev]*0.7945, fL[now])
		}
	}

	// The "present hour" is p.hour + 1: the method is based on the Fortran
	// program FTZ() where UTC 1 means 1:00 to 1:59, while elsewhere the index
	// p.hour = 0 stands for UTC 1.
	now = (p.hour + 1 + 24) % 24

	p.fL = fL[now]
}

// winterAnomaly calculates the winter anomaly factor for long paths (more than
// 9000 km) from Table 5 of P.533; values other than at 60 degrees latitude are
// interpolated.
func winterAnomaly(lat float64, month int) float64 {
	var iNS int

	aw := [12][2]float64{
		{0.30, 0.00}, // January
		{0.15, 0.00}, // February
		{0.03, 0.00}, // March
		{0.00, 0.03}, // April
		{0.00, 0.15}, // May
		{0.00, 0.30}, // June
		{0.00, 0.30}, // July
		{0.00, 0.15}, // August
		{0.00, 0.03}, // September
		{0.03, 0.00}, // October
		{0.15, 0.00}, // November
		{0.30, 0.00}, // December
	}

	if lat < 0.0 {
		iNS = south
	} else {
		iNS = north
	}

	lat = math.Abs(lat) // all latitudes must be positive

	if (lat >= 0.0 && lat <= 30.0*d2r) || lat >= 90.0*d2r {
		return 0.0
	} else if lat < 60.0*d2r {
		// Interpolate with the peak winter anomaly factor at 60 degrees.
		return aw[month][iNS] * (lat*r2d - 30.0) / 30.0
	}
	return aw[month][iNS] * (90.0 - lat*r2d) / 30.0
}

// antennaGain08 determines the largest antenna gain in the range 0 to 8
// degrees elevation and the elevation at which it occurs. The minimum
// elevation of the long model is 3 degrees, but per Dambolt and Suessmann the
// maximum between 0 and 8 degrees is used.
func antennaGain08(p *pathData, ant *antenna, direction int, elevation *float64) float64 {
	gmax := tinyDB
	for i := 0; i < 9; i++ {
		delta := float64(i) * d2r
		g := antennaGain(p, ant, delta, direction)
		if g > gmax {
			gmax = g
			*elevation = float64(i) * d2r
		}
	}

	return gmax
}

// copyCP copies the control point thisCP to thatCP.
func copyCP(thisCP controlPt, thatCP *controlPt) {
	thatCP.dip[0] = thisCP.dip[0]
	thatCP.dip[1] = thisCP.dip[1]

	thatCP.distance = thisCP.distance

	thatCP.fH[0] = thisCP.fH[0]
	thatCP.fH[1] = thisCP.fH[1]

	thatCP.foE = thisCP.foE
	thatCP.foF2 = thisCP.foF2
	thatCP.hr = thisCP.hr
	thatCP.L.lat = thisCP.L.lat
	thatCP.L.lng = thisCP.L.lng
	thatCP.ltime = thisCP.ltime
	thatCP.M3kF2 = thisCP.M3kF2

	thatCP.Sun.sza = thisCP.Sun.sza
	thatCP.Sun.sha = thisCP.Sun.sha
	thatCP.Sun.decl = thisCP.Sun.decl
	thatCP.Sun.eot = thisCP.Sun.eot
	thatCP.Sun.ha = thisCP.Sun.ha
	thatCP.Sun.lsn = thisCP.Sun.lsn
	thatCP.Sun.lsr = thisCP.Sun.lsr
	thatCP.Sun.lss = thisCP.Sun.lss

	thatCP.x = thisCP.x
}
