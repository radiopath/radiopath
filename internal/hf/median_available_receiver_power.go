package hf

import "math"

// medianAvailableReceiverPower calculates the median available receiver power
// of P.533 section 6 and the path field strength Ep.
func medianAvailableReceiverPower(p *pathData) {
	var sumPr, grw float64

	prw := tinyDB
	elevation := 2.0 * pi

	// The receiver gain Grw is calculated at the receiver elevation angles
	// for paths up to 7000 km; for longer paths the largest gain between 0
	// and 8 degrees is used.
	if p.distance <= 7000.0 {
		// The sum of the mode powers, equation (38).
		sumPr = 0.0

		// The available signal power Prw (dBW) of each mode from the field
		// strength Ew, the frequency and the receive antenna gain Grw.

		// E modes, see "Modes considered" in section 5.2.1.
		if p.n0_E != noLowestMode {
			for i := p.n0_E; i < maxEMds; i++ {
				if (i == p.n0_E && p.distance/float64(p.n0_E+1) <= 2000.0) ||
					(i != p.n0_E && p.Md_E[i].BMUF != 0.0) {
					// The receiver gain for this mode.
					p.Md_E[i].Grw = antennaGain(p, &p.A_rx, p.Md_E[i].ele, rxToTx)

					p.Md_E[i].Prw = p.Md_E[i].Ew + p.Md_E[i].Grw -
						20.0*math.Log10(p.frequency) - 107.2

					// Is this the greatest received power?
					if prw < p.Md_E[i].Prw {
						prw = p.Md_E[i].Prw

						// The dominant mode.
						p.DMptr = &p.Md_E[i]
						p.DMidx = i
					}

					sumPr += math.Pow(10.0, p.Md_E[i].Prw/10.0)
				}
			}
		}
		// F2 modes
		if p.n0_F2 != noLowestMode {
			for i := p.n0_F2; i < maxF2Mds; i++ {
				if (i == p.n0_F2 && p.distance/float64(p.n0_F2+1) <= p.dmax && p.Md_F2[i].fs < p.frequency) ||
					(i != p.n0_F2 && p.Md_F2[i].BMUF != 0.0 && p.Md_F2[i].fs < p.frequency) {
					p.Md_F2[i].Grw = antennaGain(p, &p.A_rx, p.Md_F2[i].ele, rxToTx)

					p.Md_F2[i].Prw = p.Md_F2[i].Ew + p.Md_F2[i].Grw -
						20.0*math.Log10(p.frequency) - 107.2

					// If there was an E mode Prw is already set to its power.
					if prw < p.Md_F2[i].Prw {
						prw = p.Md_F2[i].Prw

						p.DMptr = &p.Md_F2[i]
						p.DMidx = i + 3
					}

					sumPr += math.Pow(10.0, p.Md_F2[i].Prw/10.0)
				}
			}
		}

		// The total received power; with a zero sum use something small.
		if sumPr != 0.0 && p.DMptr != nil {
			p.Pr = 10.0 * math.Log10(sumPr)
			// The dominant mode is known: set the values relevant to it.
			dominantMode(p)
		} else {
			// There are no E modes and all the F2 modes are screened.
			p.Pr = tinyDB
		}

		// The path field strength is Es, which includes E layer screening.
		p.Ep = p.Es
	} else if 7000.0 < p.distance && p.distance < 9000.0 {
		grw = antennaGain08(p, &p.A_rx, rxToTx, &elevation)

		// The interpolated power Ei.
		p.Pr = p.Ei + grw - 20.0*math.Log10(p.frequency) - 107.2

		p.Grw = grw

		p.Ep = p.Ei

		// The rx antenna elevation of the long path.
		p.ele = elevation
	} else { // p.distance >= 9000.0
		grw = antennaGain08(p, &p.A_rx, rxToTx, &elevation)

		// The combined mode power El and the antenna gain between 0 and 8
		// degrees.
		p.Pr = p.El + grw - 20.0*math.Log10(p.frequency) - 107.2

		p.Grw = grw

		p.Ep = p.El

		p.ele = elevation
	}
}

// dominantMode stores the values of the dominant mode in the path. This is
// not part of P.533 but provides continuity in the analysis.
func dominantMode(p *pathData) {
	// The dominant mode represents the median path behavior: its receiver
	// gain and elevation angle.
	p.Grw = p.DMptr.Grw

	p.ele = p.DMptr.ele
}
