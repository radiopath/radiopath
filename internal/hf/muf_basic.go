package hf

import "math"

// mufBasic determines the basic MUF of the F2 modes (P.533 section 3.5) and
// the E modes (section 3.3) and stores the mode information in the path. It
// also determines the control points T + d0/2 and R - d0/2 when needed.
func mufBasic(p *pathData) {
	var n0 int
	var n int

	var hr float64 // mirror reflection height
	var dh float64 // the hop distance
	var aoi float64
	var n0F2DMUF float64 // lowest-order mode basic MUF
	var f2DMUF [2]float64
	var mn0 [2]float64 // MUF factor of the lowest-order mode
	var mn [2]float64  // MUF factor of the higher-order modes
	var fracd float64
	var i110 float64 // incident angle for a reflection height of 110 km
	var dmax float64
	var delta float64

	// Only for paths up to 9000 km.
	if p.distance > 9000 {
		return
	}

	// F2 layer basic MUF: mirror reflection height at the mid-point.
	hr = cMin(1490.0/p.CP[mp].M3kF2-176.0, 500.0)

	p.CP[mp].hr = hr

	// The lowest-order mode follows from geometry on a spherical Earth: the
	// horizon at the reflection height decides whether multiple hops are
	// necessary.
	minele := minEleAngleS * d2r
	// The angle of incidence for the minimum elevation and the height.
	aoi = incidenceAngle(minele, hr)
	// Arc length of the half hop times two is the hop length.
	dh = (pi - aoi - ((pi / 2.0) + minele)) * r0 * 2.0

	// The hop distance cannot be longer than 4000 km.
	dh = cMin(dh, 4000.0)

	for n0 = 0; n0 < maxF2Mds; n0++ {
		if dh > p.distance/float64(n0+1) { // is the horizon shorter than the hop?
			p.n0_F2 = n0
			break
		}
	}

	// Sections 3.5.1 and 3.5.2 depend on the lowest-order F2 mode existing.
	if p.n0_F2 != noLowestMode {
		// dmax at the mid-point, limited to 4000 km; used elsewhere in p533().
		p.dmax = cMin(calcdmax(&p.CP[mp]), 4000.0)

		dmax = p.dmax

		// 3.5.1 Lowest-order mode
		if p.distance <= p.dmax { // 3.5.1.1 Paths up to dmax (km)
			n0F2DMUF = calcF2DMUF(&p.CP[mp], p.distance/(float64(n0)+1.0), p.dmax, calcB(&p.CP[mp]))

			p.Md_F2[p.n0_F2].BMUF = n0F2DMUF

			// The basic F2 MUF of the path.
			p.BMUF = n0F2DMUF

			// There are no T + d0/2 and R - d0/2 control points for this path.
		} else { // 3.5.1.2 Paths longer than dmax (km)
			// The control points T + d0/2 and R - d0/2 depend on n0.
			fracd = (1.0 / (2.0 * float64(n0+1)))
			greatCirclePoint(p.L_tx, p.L_rx, &p.CP[td02], p.distance, fracd)
			fracd = (1.0 - (1.0 / (2.0 * float64(n0+1))))
			greatCirclePoint(p.L_tx, p.L_rx, &p.CP[rd02], p.distance, fracd)

			calculateCPParameters(p, &p.CP[td02])
			calculateCPParameters(p, &p.CP[rd02])

			// The F2 basic MUF at each control point, for F2(dmax)MUF.
			f2DMUF[0] = calcF2DMUF(&p.CP[td02], p.distance/(float64(n0)+1.0), dmax, calcB(&p.CP[td02]))
			f2DMUF[1] = calcF2DMUF(&p.CP[rd02], p.distance/(float64(n0)+1.0), dmax, calcB(&p.CP[rd02]))

			// The lower of the two control point MUFs.
			p.Md_F2[n0].BMUF = cMin(f2DMUF[0], f2DMUF[1])

			p.BMUF = p.Md_F2[n0].BMUF
		}

		// 3.5.2 Higher-order modes (paths up to 9000 km)
		if p.distance <= 9000 {
			for n = n0 + 1; n < maxF2Mds; n++ {
				if p.distance <= p.dmax { // 3.5.2.1 Paths up to dmax (km)
					p.Md_F2[n].BMUF = calcF2DMUF(&p.CP[mp], p.distance/(float64(n)+1.0), dmax, calcB(&p.CP[mp]))
				} else { // 3.5.2.2 Paths longer than dmax (km)
					// The mode MUF follows from the lowest-order mode MUF and the
					// scaling factor Mn/Mn0 = nF2(D)MUF/n0F2(D)MUF. Here dmax is
					// allowed to exceed 4000 km.
					mn0[0] = calcF2DMUF(&p.CP[td02], p.distance/(float64(n0)+1.0), calcdmax(&p.CP[td02]), calcB(&p.CP[td02]))
					mn0[1] = calcF2DMUF(&p.CP[rd02], p.distance/(float64(n0)+1.0), calcdmax(&p.CP[rd02]), calcB(&p.CP[rd02]))

					mn[0] = calcF2DMUF(&p.CP[td02], p.distance/(float64(n)+1.0), calcdmax(&p.CP[td02]), calcB(&p.CP[td02]))
					mn[1] = calcF2DMUF(&p.CP[rd02], p.distance/(float64(n)+1.0), calcdmax(&p.CP[rd02]), calcB(&p.CP[rd02]))

					p.Md_F2[n].BMUF = p.BMUF * cMin(mn[0]/mn0[0], mn[1]/mn0[1])
				}
			}
		}
	}

	// E layer basic MUF
	if p.distance < 4000.0 {
		// The mirror reflection height is 110 km.
		hr = 110.0

		delta = minele

		aoi = incidenceAngle(delta, hr)

		dh = (pi - aoi - ((pi / 2.0) + minele)) * r0 * 2.0

		dh = cMin(dh, 4000.0)

		for n0 = 0; n0 < 3; n0++ {
			if dh > p.distance/float64(n0+1) {
				p.n0_E = n0
				break
			}
		}

		// Is there a lowest order E mode?
		if p.n0_E != noLowestMode {
			// There are three E paths.
			for n = n0; n < maxEMds; n++ {
				// The reflection height is always 110 km for E layers.
				p.Md_E[n].hr = hr
				// The hop length of this mode.
				dh = cMin(p.distance/float64(n+1), 4000.0)
				// The elevation angle.
				delta = elevationAngle(dh, hr)
				// The incident angle for hr = 110 km.
				i110 = incidenceAngle(delta, hr)
				// The control point is the mid-point.
				if p.distance < 2000.0 {
					p.Md_E[n].BMUF = p.CP[mp].foE / math.Cos(i110)
				} else if p.distance >= 2000.0 && p.distance <= 4000.0 {
					p.Md_E[n].BMUF = cMin(p.CP[r1k].foE, p.CP[t1k].foE) / math.Cos(i110)
				} else { // p.distance > 4000.0
					p.Md_E[n].BMUF = 0.0
				}

				// The lowest-order E layer mode.
				if p.Md_E[n].BMUF != 0.0 && p.n0_E == noLowestMode {
					p.n0_E = n
				}
			}
		}
		// There are no E layer modes for distance >= 4000 km.
	}

	// Finally, the path basic MUF.
	if p.n0_E != noLowestMode {
		if p.n0_F2 != noLowestMode { // F2 and E modes exist
			p.BMUF = cMax(p.Md_E[p.n0_E].BMUF, p.Md_F2[p.n0_F2].BMUF)
		} else { // E modes exist, F2 modes do not
			p.BMUF = p.Md_E[p.n0_E].BMUF
		}
	} else {
		if p.n0_F2 != noLowestMode { // F2 modes exist, E modes do not
			p.BMUF = p.Md_F2[p.n0_F2].BMUF
		} else {
			// This is an error condition.
			p.BMUF = tooBig
		}
	}
}

// calcdmax finds the dmax of equation (5) of P.533 for a control point. It can
// be greater than 4000 km, which is only used for higher order MUFs.
func calcdmax(cp *controlPt) float64 {
	b := calcB(cp)

	dmax := 4780.0 + (12610.0+(2140.0/math.Pow(cp.x, 2.0))-(49720.0/math.Pow(cp.x, 4.0))+(688900.0/math.Pow(cp.x, 6.0)))*((1.0/b)-0.303)

	return dmax
}

// calcB determines the intermediate value B of equation (6) of P.533 and sets
// x, the foF2/foE ratio of the control point.
func calcB(cp *controlPt) float64 {
	// x is the ratio of foF2 to foE.
	if cp.foE != 0.0 { // does foE exist?
		cp.x = cMax(cp.foF2/cp.foE, 2.0)
	} else {
		cp.x = 2.0
	}

	b := cp.M3kF2 - 0.124 + (math.Pow(cp.M3kF2, 2)-4)*(0.0215+0.005*math.Sin((7.854/cp.x)-1.9635))

	return b
}

// calcCd performs equation (4) of P.533.
func calcCd(d, dmax float64) float64 {
	z := 1.0 - 2.0*d/dmax

	cd := 0.74 - 0.591*z - 0.424*math.Pow(z, 2) - 0.090*math.Pow(z, 3) + 0.088*math.Pow(z, 4) + 0.181*math.Pow(z, 5) + 0.096*math.Pow(z, 6)

	return cd
}

// calcF2DMUF determines the F2 layer MUF of equation (3) of P.533.
func calcF2DMUF(cp *controlPt, distance, dmax, b float64) float64 {
	var d float64

	// If the distance is less than dmax use the distance.
	if distance <= dmax {
		d = distance
	} else {
		d = dmax
	}

	// Equation (4)
	cd := calcCd(d, dmax)

	d = 3000.0 // C3000: value of Cd for D = 3000 km

	c3k := calcCd(d, dmax)

	f2DMUF := (1.0+(cd/c3k)*(b-1.0))*cp.foF2 + (cp.fH[hr300km]/2.0)*(1.0-(distance/dmax))

	return f2DMUF
}
