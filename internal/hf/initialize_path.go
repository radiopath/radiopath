package hf

// initializePath sets the output values of the path to their defaults,
// determines the distance and the control points that are known up front
// (mid-path, T + 1000 km and R - 1000 km) and the season. validatePath() must
// have succeeded before.
func initializePath(p *pathData) {
	// Clear the variables that are determined elsewhere.
	p.B = 99.9
	p.BCR = 0.0
	p.BMUF = 99.9
	p.DMidx = noDominantMode
	p.DMptr = nil
	p.DlSI = tinyDB
	p.DlSN = tinyDB
	p.DuSI = tinyDB
	p.DuSN = tinyDB
	p.EIRP = tinyDB
	p.Ei = tinyDB
	p.El = tinyDB
	p.E0 = tinyDB
	p.Ep = tinyDB
	p.Es = tinyDB
	p.F = tinyDB
	p.fH = 0.0
	p.Gap = tinyDB
	p.Grw = tinyDB
	p.Gtl = tinyDB
	p.K[0] = 0.0
	p.K[1] = 0.0
	p.Ly = 0.0
	p.MIR = tinyDB
	p.MUF10 = 99.9
	p.MUF50 = 99.9
	p.MUF90 = 99.9
	p.OCR = 0.0
	p.OCRs = 0.0
	p.OPMUF = 99.9
	p.OPMUF10 = 99.9
	p.OPMUF90 = 99.9
	p.Pr = tinyDB
	p.RF = 0.0
	p.RSN = 0.0
	p.RT = 0.0
	p.SIR = tinyDB
	p.SNR = tinyDB
	p.SNRXX = tinyDB
	p.distance = 999999.9
	p.dmax = 999999.9
	p.ele = 2.0 * pi
	p.fL = 0.0
	p.fM = 0.0
	p.n0_E = noLowestMode
	p.n0_F2 = noLowestMode
	p.probocc = 0.0
	p.ptick = 0.0
	p.season = 99

	initializeModes(p.Md_F2[:])
	initializeModes(p.Md_E[:])

	// The distance between the tx and rx is needed for the control points.
	p.distance = greatCircleDistance(p.L_tx, p.L_rx)

	// Degenerate case of a zero distance: use epsilon as an approximation.
	if p.distance == 0.0 {
		p.distance = dblEpsilon
	}

	// On a long path the distance is the rest of the great circle.
	if p.SorL == longPath {
		p.distance = r0*pi*2 - p.distance
	}

	initializeCPs(p)

	p.noiseP.Initialize()

	// Several calculations need the season.
	p.season = whatSeason(p.CP[mp].L, p.month)
}

// initializeCPs zeros the five potential control points and determines the
// location and ionospheric properties of the mid-point and, for paths of at
// least 2000 km, of the points 1000 km from either end. T + d0/2 and R - d0/2
// need the lowest-order mode and are determined in mufBasic().
func initializeCPs(p *pathData) {
	for i := 0; i < 5; i++ {
		p.CP[i].L.lat = 0.0
		p.CP[i].L.lng = 0.0
		p.CP[i].distance = 0.0
		p.CP[i].foE = 0.0
		p.CP[i].foF2 = 0.0
		p.CP[i].M3kF2 = 0.0
		p.CP[i].fH[hr300km] = 0.0
		p.CP[i].dip[hr300km] = 0.0
		p.CP[i].fH[hr100km] = 0.0
		p.CP[i].dip[hr100km] = 0.0
		p.CP[i].Sun.lsn = 0.0
		p.CP[i].Sun.lsr = 0.0
		p.CP[i].Sun.lss = 0.0
		p.CP[i].Sun.sza = 0.0
		p.CP[i].Sun.decl = 0.0
		p.CP[i].Sun.eot = 0.0
		p.CP[i].Sun.sha = 0.0
		p.CP[i].Sun.ha = 0.0
		p.CP[i].ltime = 0.0
		p.CP[i].hr = 0.0
		p.CP[i].x = 0.0
	}

	// Mid-point control point, always used. All distances are relative to
	// the transmitter.
	fracd := 0.5
	greatCirclePoint(p.L_tx, p.L_rx, &p.CP[mp], p.distance, fracd)
	calculateCPParameters(p, &p.CP[mp])

	// There is no point in control points 1000 km from the ends of a path
	// shorter than 2000 km.
	if p.distance >= 2000.0 {
		fracd = (p.distance - 1000) / p.distance
		greatCirclePoint(p.L_tx, p.L_rx, &p.CP[r1k], p.distance, fracd)

		fracd = 1000.0 / p.distance
		greatCirclePoint(p.L_tx, p.L_rx, &p.CP[t1k], p.distance, fracd)

		calculateCPParameters(p, &p.CP[t1k])
		calculateCPParameters(p, &p.CP[r1k])
	}
}

// initializeModes sets n modes to their defaults.
func initializeModes(m []mode) {
	for i := range m {
		m[i].BMUF = 0.0
		m[i].MUF90 = 0.0
		m[i].MUF50 = 0.0
		m[i].MUF10 = 0.0
		m[i].OPMUF = 0.0
		m[i].OPMUF10 = 0.0
		m[i].OPMUF90 = 0.0
		m[i].Fprob = 0.0
		m[i].deltal = 0.0
		m[i].deltau = 0.0
		m[i].fs = 0.0
		m[i].Lb = -tinyDB
		m[i].Ew = tinyDB
		m[i].Prw = tinyDB
		m[i].Grw = tinyDB
		m[i].hr = 0.0
		m[i].tau = 0.0
		m[i].ele = 0.0
		m[i].MC = 0
	}
}

// whatSeason determines the season index (month and latitude dependent) that
// locates the variables in the foF2var array.
func whatSeason(l location, month int) int {
	season := winter

	if l.lat >= 0 { // Northern hemisphere and the equator
		switch month {
		case nov, dec, jan, feb:
			season = winter
		case mar, apr, sep, oct:
			season = equinox
		case may, jun, jul, aug:
			season = summer
		}
	} else { // Southern hemisphere
		switch month {
		case may, jun, jul, aug:
			season = winter
		case mar, apr, sep, oct:
			season = equinox
		case nov, dec, jan, feb:
			season = summer
		}
	}

	return season
}
