package hf

import "math"

const (
	noIL = 9.14 // "Not otherwise included" loss of the short model (dB)
	// pen selects the absorption method: penetration points (true) or the
	// control point method of P.533-12 (false).
	pen = true
)

// medianSkywaveFieldStrengthShort calculates the median sky-wave field
// strength of P.533 section 5.2 (paths up to 7000 km, also used up to 9000 km
// for the interpolation). The E and F2 modes are kept in separate loops on
// purpose, to follow the flow of the standard.
func medianSkywaveFieldStrengthShort(p *pathData) {
	var dh, delta, psi float64
	var hrE, hrF2 float64
	var fv, pt, li, gt, aoi110, at, fL, lm, lg, lh, etw float64

	// Only for paths up to 9000 km.
	if p.distance > 9000 {
		return
	}

	// 5.2 Paths up to 7000 km. The SSN is restricted to 160.
	ssn := float64(cMinInt(p.SSN, maxSSN))

	// The mirror reflection height of the E layer.
	hrE = 110.0

	// The mirror reflection height of the F2 layers.
	if p.distance > p.dmax { // distance is less than 9000 km and greater than dmax
		// The smallest foF2 amongst the control points.
		hrF2 = cMin((1490.0/p.CP[smallestCPfoF2(p)].M3kF2)-176.0, 500.0)
	} else {
		// The mid-point.
		hrF2 = p.CP[mp].hr
	}

	// The local time at the mid-path point.
	tz := int(p.CP[mp].L.lng / (15.0 * d2r))
	mpltime := int(cFmod(p.CP[mp].ltime+float64(tz), 24))

	// E modes
	if p.n0_E != noLowestMode {
		for n := p.n0_E; n < maxEMds; n++ {
			if (n == p.n0_E && p.distance/(float64(p.n0_E)+1.0) <= 2000.0) || // lowest order mode shorter than 2000 km
				(n > p.n0_E && p.Md_E[n].BMUF != 0.0) { // higher order modes
				fn := float64(n)

				// The elevation angle from equation 13 of section 5.1.
				delta = elevationAngle(p.distance/(fn+1.0), hrE)

				p.Md_E[n].ele = delta

				// The angle of incidence at the height hr = 110 km.
				aoi110 = incidenceAngle(delta, hrE)

				// The vertical-incidence wave frequency.
				fv = p.frequency * math.Cos(aoi110)

				// The hop distance.
				dh = p.distance / (fn + 1.0)

				// The virtual slant range (19).
				psi = dh / (2.0 * r0)

				p.ptick = math.Abs(2.0*r0*(math.Sin(psi)/math.Cos(delta+psi))) * (fn + 1.0)

				if p.distance <= 2000.0 {
					if pen {
						at = penetrationPoints(p, fn, hrE, fv)
					} else {
						at = absorptionTerm(p.CP[mp], p.month, fv)
					}

					// The longitudinal gyrofrequency.
					fL = math.Abs(p.CP[mp].fH[hr100km] * math.Sin(p.CP[mp].dip[hr100km]))

					// The auroral and other signal losses.
					lh = findLh(p.CP[mp], dh, mpltime, p.month)
				} else { // three control points
					if pen {
						at = penetrationPoints(p, fn, hrE, fv)
					} else {
						at = (absorptionTerm(p.CP[mp], p.month, fv) +
							absorptionTerm(p.CP[t1k], p.month, fv) +
							absorptionTerm(p.CP[r1k], p.month, fv)) / 3.0
					}

					// The average longitudinal gyrofrequency.
					fL = (math.Abs(p.CP[mp].fH[hr100km]*math.Sin(p.CP[mp].dip[hr100km])) +
						math.Abs(p.CP[t1k].fH[hr100km]*math.Sin(p.CP[t1k].dip[hr100km])) +
						math.Abs(p.CP[r1k].fH[hr100km]*math.Sin(p.CP[r1k].dip[hr100km]))) / 3.0

					lh = (findLh(p.CP[mp], dh, mpltime, p.month) +
						findLh(p.CP[t1k], dh, mpltime, p.month) +
						findLh(p.CP[r1k], dh, mpltime, p.month)) / 3.0
				}

				// The absorption loss (dB) of the n-hop mode.
				li = ((fn + 1.0) * (1.0 + 0.0067*ssn) * at) / (math.Pow((p.frequency+fL), 2) * math.Cos(aoi110))

				// The "above-the-MUF" loss.
				if p.frequency <= p.Md_E[n].BMUF {
					lm = 0.0
				} else {
					lm = cMin(46.0*math.Pow(((p.frequency/p.Md_E[n].BMUF)-1.0), 0.5)+5, 58.0)
				}

				// The ground reflection loss; n is a C index starting at 0.
				lg = 2.0 * ((fn + 1.0) - 1.0)

				// The "not otherwise included" loss.
				p.Lz = noIL

				// The basic transmission loss of the mode.
				p.Md_E[n].Lb = 32.45 + 20.0*math.Log10(p.frequency) + 20.0*math.Log10(p.ptick) + li + lm + lg + lh + p.Lz

				// The Tx antenna gain in the desired direction (dB).
				gt = antennaGain(p, &p.A_tx, delta, txToRx)

				pt = p.txpower

				p.Md_E[n].Ew = 136.6 + pt + gt + 20.0*math.Log10(p.frequency) - p.Md_E[n].Lb
			} else {
				break // no more E modes that satisfy the criteria
			}
		}
	}

	// F2 modes
	if p.n0_F2 != noLowestMode {
		for n := p.n0_F2; n < maxF2Mds; n++ {
			// Modes considered: any mode with an E layer screening frequency
			// below the operating frequency; the lowest order mode must also
			// have hops shorter than dmax.
			if (n == p.n0_F2 && p.distance/(float64(p.n0_F2)+1.0) <= p.dmax && p.Md_F2[n].fs < p.frequency) ||
				((n > p.n0_F2 && p.Md_F2[n].BMUF != 0.0) && p.Md_F2[n].fs < p.frequency) {
				fn := float64(n)

				delta = elevationAngle(p.distance/(fn+1.0), hrF2)

				p.Md_F2[n].ele = delta

				aoi110 = incidenceAngle(delta, 110.0)

				fv = p.frequency * math.Cos(aoi110)

				dh = p.distance / (fn + 1.0)

				psi = dh / (2.0 * r0)

				p.ptick = math.Abs(2.0*r0*(math.Sin(psi)/math.Cos(delta+psi))) * (fn + 1.0)

				if p.distance <= 2000.0 { // the mid-path control point
					if pen {
						at = penetrationPoints(p, fn, hrF2, fv)
					} else {
						at = absorptionTerm(p.CP[mp], p.month, fv)
					}

					fL = math.Abs(p.CP[mp].fH[hr100km] * math.Sin(p.CP[mp].dip[hr100km]))

					lh = findLh(p.CP[mp], dh, mpltime, p.month)
				} else if 2000.0 < p.distance && p.distance <= p.dmax { // three control points
					if pen {
						at = penetrationPoints(p, fn, hrF2, fv)
					} else {
						at = (absorptionTerm(p.CP[mp], p.month, fv) +
							absorptionTerm(p.CP[t1k], p.month, fv) +
							absorptionTerm(p.CP[r1k], p.month, fv)) / 3.0
					}

					fL = (math.Abs(p.CP[mp].fH[hr100km]*math.Sin(p.CP[mp].dip[hr100km])) +
						math.Abs(p.CP[t1k].fH[hr100km]*math.Sin(p.CP[t1k].dip[hr100km])) +
						math.Abs(p.CP[r1k].fH[hr100km]*math.Sin(p.CP[r1k].dip[hr100km]))) / 3.0

					lh = (findLh(p.CP[mp], dh, mpltime, p.month) +
						findLh(p.CP[t1k], dh, mpltime, p.month) +
						findLh(p.CP[r1k], dh, mpltime, p.month)) / 3.0
				} else { // five control points
					if pen {
						at = penetrationPoints(p, fn, hrF2, fv)
					} else {
						at = (absorptionTerm(p.CP[mp], p.month, fv) +
							absorptionTerm(p.CP[t1k], p.month, fv) +
							absorptionTerm(p.CP[r1k], p.month, fv) +
							absorptionTerm(p.CP[td02], p.month, fv) +
							absorptionTerm(p.CP[rd02], p.month, fv)) / 5.0
					}

					fL = (math.Abs(p.CP[mp].fH[hr100km]*math.Sin(p.CP[mp].dip[hr100km])) +
						math.Abs(p.CP[t1k].fH[hr100km]*math.Sin(p.CP[t1k].dip[hr100km])) +
						math.Abs(p.CP[r1k].fH[hr100km]*math.Sin(p.CP[r1k].dip[hr100km])) +
						math.Abs(p.CP[td02].fH[hr100km]*math.Sin(p.CP[td02].dip[hr100km])) +
						math.Abs(p.CP[rd02].fH[hr100km]*math.Sin(p.CP[rd02].dip[hr100km]))) / 5.0

					lh = (findLh(p.CP[mp], dh, mpltime, p.month) +
						findLh(p.CP[t1k], dh, mpltime, p.month) +
						findLh(p.CP[r1k], dh, mpltime, p.month) +
						findLh(p.CP[td02], dh, mpltime, p.month) +
						findLh(p.CP[rd02], dh, mpltime, p.month)) / 5.0
				}

				li = ((fn + 1.0) * (1.0 + 0.0067*ssn) * at) / (math.Pow((p.frequency+fL), 2) * math.Cos(aoi110))

				// The "above-the-MUF" loss.
				if p.frequency <= p.Md_F2[n].BMUF {
					lm = 0.0
				} else {
					if p.distance <= 3000 {
						lm = cMin(36.0*math.Pow(((p.frequency/p.Md_F2[n].BMUF)-1.0), 0.5)+5.0, 60.0)
					} else {
						lm = cMin(70.0*(p.frequency/p.Md_F2[n].BMUF-1.0)+8, 80.0)
					}
				}

				lg = 2.0 * ((fn + 1.0) - 1.0)

				p.Lz = noIL

				// The total loss.
				p.Md_F2[n].Lb = 32.45 + 20.0*math.Log10(p.frequency) + 20.0*math.Log10(p.ptick) + li + lm + lg + lh + p.Lz

				gt = antennaGain(p, &p.A_tx, delta, txToRx)

				pt = p.txpower

				p.Md_F2[n].Ew = 136.6 + pt + gt + 20.0*math.Log10(p.frequency) - p.Md_F2[n].Lb
			}
		}
	}

	// The overall resultant equivalent median sky-wave field strength Es, see
	// "Modes considered" in section 5.2.1. Es is re-initialised for clarity.
	p.Es = tinyDB
	etw = 0.0

	// Sum the lowest-order E mode with a hop length up to 2000 km and the next
	// two higher-order E modes that exist.
	if p.n0_E != noLowestMode {
		for n := p.n0_E; n < maxEMds; n++ {
			if (n == p.n0_E && p.distance/float64(p.n0_E+1) <= 2000.0) ||
				(n != p.n0_E && p.Md_E[n].BMUF != 0.0) {
				etw += math.Pow(10.0, (p.Md_E[n].Ew / 10.0))
				p.Md_E[n].MC = 1
			}
		}
	}

	// The lowest-order F2 mode with a hop length up to dmax and maximally the
	// next five higher-order modes, if the E layer screening frequency of the
	// mode is less than the operating frequency.
	if p.n0_F2 != noLowestMode {
		for n := p.n0_F2; n < maxF2Mds; n++ {
			if (n == p.n0_F2 && p.distance/float64(p.n0_F2+1) <= p.dmax && p.Md_F2[n].fs < p.frequency) ||
				(n != p.n0_F2 && p.Md_F2[n].BMUF != 0.0 && p.Md_F2[n].fs < p.frequency) {
				etw += math.Pow(10.0, (p.Md_F2[n].Ew / 10.0))
				p.Md_F2[n].MC = 1
			}
		}
	}

	// The field strength if there are any modes to consider; otherwise Es
	// remains TINYDB.
	if etw != 0.0 {
		p.Es = 10.0 * math.Log10(etw) // field strength with E layer screening
	}
}

// absorptionTerm calculates the absorption term of equation (20) of P.533 for
// a control point: the product of the absorption factor at noon, the
// absorption layer penetration factor and the diurnal variation.
func absorptionTerm(cp controlPt, month int, fv float64) float64 {
	var cp0 controlPt // temporary control point

	zeroCP(&cp0)

	// The diurnal absorption exponent.
	pe := diurnalAbsorptionExponent(cp, month)

	// The solar zenith angle at the control point, at most 102 degrees.
	chij := cMin(cp.Sun.sza, 102.0*d2r)

	fchij := cMax(math.Pow(math.Cos(0.881*chij), pe), 0.02)

	// The solar zenith angle at local noon: the hour handed to
	// solarParams() is the UTC hour of local noon found by
	// calculateCPParameters().
	cp0 = cp
	solarParams(&cp0, month, cp.Sun.lsn)

	chijnoon := cp0.Sun.sza

	fchijnoon := cMax(math.Pow(math.Cos(0.881*chijnoon), pe), 0.02)

	// The remaining absorption parameters.
	atnoon := absorptionFactor(cp, month)

	phin := absorptionLayerPenetrationFactor(fv / cp.foE)

	return atnoon * phin * fchij / fchijnoon
}

// diurnalAbsorptionExponent determines the diurnal absorption exponent p of
// Figure 3 of P.533 as a function of the month and the magnetic dip. It is
// based on CONTP() of REC533.
func diurnalAbsorptionExponent(cp controlPt, month int) float64 {
	var a float64
	var i int

	// The exponent p
	pe := 0.0

	// The modified magnetic dip angle (degrees as noted in the C code, but
	// the value is in radians).
	moddip := math.Abs(math.Atan2(cp.dip[hr100km], math.Sqrt(math.Cos(cp.L.lat))))

	if moddip > 70.0*d2r {
		moddip = 70.0 * d2r
	}

	if cp.L.lat < 0.0 {
		month = month + 6
		if month > 11 {
			month = month - 12
		}
	}

	pp := diurnalPpt[month] * d2r

	if moddip > pp {
		i = 1
		moddip = -1.0 + 2.0*(moddip-pp)/(70.0*d2r-pp)
	} else {
		i = 0
		moddip = -1.0 + 2.0*moddip/pp
	}

	sx := 1.0

	for j := 0; j < 7; j++ {
		if month <= 5 {
			a = diurnalPval1[month][i][j]
		} else {
			a = diurnalPval2[month-6][i][j]
		}

		pe = pe + a*sx
		sx = sx * moddip
	}

	return pe
}

// absorptionFactor calculates the absorption factor ATnoon of Figure 1 of
// P.533 (local noon, R12 = 0). It is based on CONTAT() of REC533.
func absorptionFactor(cp controlPt, month int) float64 {
	// Month index into the table:
	// Jan Feb Mar Apr May Jun Jul Aug Sep Oct Nov Dec
	//  0   1   2   3   4   5   5   4   6   7   8   0
	i := month

	switch i {
	case jul:
		i = 5
	case aug:
		i = 4
	case sep:
		i = 6
	case oct:
		i = 7
	case nov:
		i = 8
	case dec:
		i = 0
	}

	x := math.Abs(cp.L.lat * r2d)
	if x >= 70.0 {
		x = 69.99 // keeps j below 28 after the cast to int
	}
	x = x / 2.5
	j := int(x)
	x = x - float64(j)
	atnoon := absorptionATNO[i][j+1]*x + absorptionATNO[i][j]*(1.0-x)

	return atnoon
}

// absorptionLayerPenetrationFactor determines the absorption layer
// penetration factor of Figure 2 of P.533 from the ratio t of the
// vertical-incidence frequency and foE. It is based on PHIFUN() of REC533.
func absorptionLayerPenetrationFactor(t float64) float64 {
	var x, phi float64

	if t <= 1.0 {
		if t < 0.0 {
			phi = 0.0
		} else {
			x = (t - 0.475) / 0.475
			phi = (((((-0.093*x+0.04)*x+0.127)*x-.027)*x+0.044)*x+0.159)*x + 0.225
			phi = cMin(phi, 0.53)
		}
	} else {
		if t <= 2.2 {
			x = (t - 1.65) / 0.55
			phi = (((((.043*x-.07)*x-.027)*x+.034)*x+.054)*x-.049)*x + .375
			phi = cMin(phi, 0.53)
		} else {
			if t <= 10.0 {
				x = t
				phi = 0.34 + (((10.0 - x) * 0.02) / 7.8)
			} else {
				phi = 0.34
			}
		}
	}

	// Multiply by the scaling factor.
	phi = phi / 0.34

	return phi
}

// findLh finds the value of Lh from Table 2 of P.533 "Values of Lh giving
// auroral and other signal losses". The first dimension of the table is the
// transmission range (up to 2500 km, above 2500 km), then season, geomagnetic
// latitude and mid-path local time in 3-hour steps.
func findLh(cp controlPt, dh float64, hour int, month int) float64 {
	var txrange, mplt, gmlat int

	// The geomagnetic coordinates of the control point.
	gn := geomagneticCoords(cp.L)

	// The season index.
	season := whatSeasonForLh(cp.L, month)

	// The transmission range index.
	if dh <= 2500.0 {
		txrange = 0
	} else {
		txrange = 1
	}

	// The mid-point local time index.
	if 1 <= hour && hour < 4 {
		mplt = 0
	} else if 4 <= hour && hour < 7 {
		mplt = 1
	} else if 7 <= hour && hour < 10 {
		mplt = 2
	} else if 10 <= hour && hour < 13 {
		mplt = 3
	} else if 13 <= hour && hour < 16 {
		mplt = 4
	} else if 16 <= hour && hour < 19 {
		mplt = 5
	} else if 19 <= hour && hour < 22 {
		mplt = 6
	} else if 22 <= hour || hour < 1 {
		mplt = 7
	}

	gn.lat = math.Abs(gn.lat)

	// The geomagnetic latitude index.
	if 77.5*d2r <= gn.lat {
		gmlat = 0
	} else if 72.5*d2r <= gn.lat && gn.lat < 77.5*d2r {
		gmlat = 1
	} else if 67.5*d2r <= gn.lat && gn.lat < 72.5*d2r {
		gmlat = 2
	} else if 62.5*d2r <= gn.lat && gn.lat < 67.5*d2r {
		gmlat = 3
	} else if 57.5*d2r <= gn.lat && gn.lat < 62.5*d2r {
		gmlat = 4
	} else if 52.5*d2r <= gn.lat && gn.lat < 57.5*d2r {
		gmlat = 5
	} else if 47.5*d2r <= gn.lat && gn.lat < 52.5*d2r {
		gmlat = 6
	} else if 42.5*d2r <= gn.lat && gn.lat < 47.5*d2r {
		gmlat = 7
	} else if 42.5*d2r > gn.lat {
		return 0.0 // nothing more to do
	}

	return auroralLh[txrange][season][gmlat][mplt]
}

// whatSeasonForLh determines the season index into the Lh table from the
// month and latitude.
func whatSeasonForLh(l location, month int) int {
	season := winter

	if l.lat >= 0 { // Northern hemisphere and the equator
		switch month {
		case dec, jan, feb:
			season = winter
		case mar, apr, may, sep, oct, nov:
			season = equinox
		case jun, jul, aug:
			season = summer
		}
	} else { // Southern hemisphere
		switch month {
		case jun, jul, aug:
			season = winter
		case mar, apr, may, sep, oct, nov:
			season = equinox
		case dec, jan, feb:
			season = summer
		}
	}

	return season
}

// smallestCPfoF2 determines the index of the control point with the smallest
// foF2. The C code sorts the indices by brute force and returns the last
// non-zero one.
func smallestCPfoF2(p *pathData) int {
	temp := 0
	idx := [5]int{0, 1, 2, 3, 4}

	// Sort by brute force.
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			if p.CP[idx[i]].foF2 > p.CP[idx[j]].foF2 {
				temp = idx[i]
				idx[i] = idx[j]
				idx[j] = temp
			}
		}
	}

	// Return the last non-zero index.
	for i := 0; i < 5; i++ {
		if idx[i] != 0 {
			temp = idx[i]
		}
	}

	return temp
}

// antennaGain finds the antenna gain at the elevation delta (radians) by
// bilinear interpolation of the pattern, for the frequency closest to the path
// frequency. The pattern is indexed in degrees; indices outside the pattern
// (negative elevations) are clamped, where the C code reads out of bounds.
func antennaGain(p *pathData, ant *antenna, delta float64, direction int) float64 {
	b := 0.0 // bearing in degrees

	freqIndex := 0
	// With pattern data for several frequencies use the closest one.
	if ant.freqn > 1 {
		minFreqDelta := tooBig
		for i := 0; i < ant.freqn; i++ {
			freqDelta := math.Abs(ant.freqs[i] - p.frequency)
			if freqDelta < minFreqDelta {
				minFreqDelta = freqDelta
				freqIndex = i
			}
		}
	}

	// delta is in radians, the pattern is indexed in degrees.
	delta = delta * r2d

	if direction == txToRx {
		b = bearing(p.L_tx, p.L_rx, p.SorL) * r2d
	} else if direction == rxToTx {
		b = bearing(p.L_rx, p.L_tx, p.SorL) * r2d
	}

	// The indices of the neighbors; the bearing might wrap around.
	deltaU := clampIndex(int(math.Ceil(delta)), antElevations)
	deltaL := clampIndex(int(math.Floor(delta)), antElevations)

	br := int(math.Ceil(b)) % 360
	bl := int(math.Floor(b)) % 360

	pat := ant.pattern[freqIndex]
	ll := pat[bl][deltaL]
	lr := pat[br][deltaL]
	ul := pat[bl][deltaU]
	ur := pat[br][deltaU]

	// The fractional row and column; the distance between indices is 1 degree.
	r := delta - float64(int(delta))
	c := b - float64(int(b))

	return bilinearInterpolation(ll, lr, ul, ur, r, c)
}

// clampIndex limits i to the range 0..n-1.
func clampIndex(i, n int) int {
	if i < 0 {
		return 0
	}
	if i > n-1 {
		return n - 1
	}
	return i
}

// zeroCP zeros a control point.
func zeroCP(cp *controlPt) {
	cp.L.lat = 0.0
	cp.L.lng = 0.0
	cp.distance = 0.0
	cp.foE = 0.0
	cp.foF2 = 0.0
	cp.M3kF2 = 0.0
	cp.fH[hr300km] = 0.0
	cp.dip[hr300km] = 0.0
	cp.fH[hr100km] = 0.0
	cp.dip[hr100km] = 0.0
	cp.Sun.lsn = 0.0
	cp.Sun.lsr = 0.0
	cp.Sun.lss = 0.0
	cp.Sun.sza = 0.0
	cp.Sun.decl = 0.0
	cp.Sun.eot = 0.0
	cp.Sun.sha = 0.0
	cp.Sun.ha = 0.0
	cp.ltime = 0.0
	cp.hr = 0.0
}

// penetrationPoints finds the 90 km penetration points of the hops (as the
// long model does) and returns the average absorption term over them. There
// are twice as many penetration points as hops.
func penetrationPoints(p *pathData, noh, hr, fv float64) float64 {
	const (
		txEnd = 0
		rxEnd = 1
	)
	var pp [2]controlPt

	// The hop distance.
	dh := p.distance / (noh + 1.0)

	// The elevation angle.
	delta := elevationAngle(dh, hr)

	// The angle of incidence at the 90 km penetration points.
	aoi90 := incidenceAngle(delta, 90.0)

	// The 90-km height half-hop distance.
	phi := (pi/2.0 - delta - aoi90)

	dh90 := r0 * phi

	atSum := 0.0 // the sum of the absorption terms

	for i := 0; float64(i) <= noh; i++ { // 90-km penetration points
		// The penetration points are calculated in pairs because the
		// fractional distance differs a little for each end.
		zeroCP(&pp[txEnd])
		zeroCP(&pp[rxEnd])

		// The end nearest the transmitter for this hop.
		fracd := (float64(i)*dh + dh90) / p.distance
		greatCirclePoint(p.L_tx, p.L_rx, &pp[txEnd], p.distance, fracd)
		calculateCPParameters(p, &pp[txEnd])

		pp[txEnd].hr = 90.0

		atSum += absorptionTerm(pp[txEnd], p.month, fv)

		// The end nearest the receiver for this hop.
		fracd = (float64(i+1)*dh - dh90) / p.distance
		greatCirclePoint(p.L_tx, p.L_rx, &pp[rxEnd], p.distance, fracd)
		calculateCPParameters(p, &pp[rxEnd])

		pp[rxEnd].hr = 90.0

		atSum += absorptionTerm(pp[rxEnd], p.month, fv)
	}

	// The average of the absorption over all penetration points.
	return atSum / (2.0 * (noh + 1))
}
