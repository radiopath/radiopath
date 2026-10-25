package hf

import "math"

// mufVariability calculates the F2 and E mode probabilities Fprob and the 50%,
// 90% and 10% MUFs (P.533 sections 3.6 and 3.7).
func mufVariability(p *pathData) {
	var decile int

	var eMUF10, eMUF90 float64
	var f2MUF10, f2MUF90 float64

	// Only for paths up to 9000 km.
	if p.distance > 9000 {
		return
	}

	// Section 3.6.5 implies that the basic MUF and the 50% MUF are the same.
	p.MUF50 = p.BMUF

	// The F2 layer modes
	for i := 0; i < maxF2Mds; i++ {
		if p.Md_F2[i].BMUF != 0.0 { // the mode exists
			p.Md_F2[i].MUF50 = p.Md_F2[i].BMUF

			// The decile factors
			decile = dl
			p.Md_F2[i].deltal = findfoF2var(p, p.CP[mp].ltime, p.CP[mp].L.lat, decile)

			decile = du
			p.Md_F2[i].deltau = findfoF2var(p, p.CP[mp].ltime, p.CP[mp].L.lat, decile)

			// The other MUFs
			p.Md_F2[i].MUF10 = p.Md_F2[i].deltau * p.Md_F2[i].MUF50
			p.Md_F2[i].MUF90 = p.Md_F2[i].deltal * p.Md_F2[i].MUF50

			// The probability that the mode is supported
			if p.frequency < p.Md_F2[i].MUF50 {
				p.Md_F2[i].Fprob = cMin(1.3-(0.8/(1.0+((1.0-(p.frequency/p.Md_F2[i].MUF50))/(1.0-p.Md_F2[i].deltal)))), 1.0)
			} else {
				p.Md_F2[i].Fprob = cMax((0.8/(1.0+(((p.frequency/p.Md_F2[i].MUF50)-1.0)/(p.Md_F2[i].deltau-1.0))))-0.3, 0.0)
			}
		}
	}

	// The same for the E layer but with the decile factors fixed by
	// section 3.6: 0.95 and 1.05.
	for i := 0; i < maxEMds; i++ {
		if p.Md_E[i].BMUF != 0.0 {
			p.Md_E[i].MUF50 = p.Md_E[i].BMUF

			p.Md_E[i].deltal = 0.95
			p.Md_E[i].deltau = 1.05

			p.Md_E[i].MUF10 = p.Md_E[i].deltau * p.Md_E[i].MUF50
			p.Md_E[i].MUF90 = p.Md_E[i].deltal * p.Md_E[i].MUF50

			if p.frequency < p.Md_E[i].MUF50 {
				p.Md_E[i].Fprob = cMin(1.3-(0.8/(1.0+((1.0-(p.frequency/p.Md_E[i].MUF50))/(1.0-p.Md_E[i].deltal)))), 1.0)
			} else {
				p.Md_E[i].Fprob = cMax((0.8/(1.0+(((p.frequency/p.Md_E[i].MUF50)-1.0)/(p.Md_E[i].deltau-1.0))))-0.3, 0.0)
			}
		}
	}

	// The largest 90% and 10% MUF of the existing modes, as for the basic MUF.
	eMUF10 = 0.0
	eMUF90 = 0.0
	for i := 0; i < maxEMds; i++ {
		if p.Md_E[i].BMUF != 0.0 {
			if p.Md_E[i].MUF90 > eMUF90 {
				eMUF90 = p.Md_E[i].MUF90
			}
			if p.Md_E[i].MUF10 > eMUF10 {
				eMUF10 = p.Md_E[i].MUF10
			}
		}
	}
	f2MUF10 = 0.0
	f2MUF90 = 0.0
	for i := 0; i < maxF2Mds; i++ {
		if p.Md_F2[i].BMUF != 0.0 {
			if p.Md_F2[i].MUF90 > f2MUF90 {
				f2MUF90 = p.Md_F2[i].MUF90
			}
			if p.Md_F2[i].MUF10 > f2MUF10 {
				f2MUF10 = p.Md_F2[i].MUF10
			}
		}
	}

	p.MUF90 = cMax(eMUF90, f2MUF90)
	p.MUF10 = cMax(eMUF10, f2MUF10)
}

// findfoF2var determines the variation of foF2 for the season of the path, the
// hour, the latitude and the decile by bilinear interpolation (ITU-R P.1144).
func findfoF2var(p *pathData, hour, lat float64, decile int) float64 {
	lat = math.Abs(lat / (5.0 * d2r)) // 5 degree increments

	// The fractional row and column; the distance between indices is 1.0.
	r := lat - float64(int(lat))
	c := hour - float64(int(hour))

	latL := int(math.Floor(lat))
	latU := int(math.Ceil(lat))

	if latL < 0 {
		latL = 18 // rollunder
		// The fractional row is reversed when negative.
		r = 1.0 - r
	}
	if latU > 18 {
		latU = 0 // rollover
	}

	hourL := int(math.Floor(hour))
	hourU := int(math.Ceil(hour))

	if hourL < 0 {
		hourL = 23 // rollunder
		c = 1.0 - c
	}

	if hourU > 23 {
		hourU = 0 // rollover
	}

	// The sunspot number index.
	var ssn int
	if p.SSN < 50 {
		ssn = 0
	} else if 50 <= p.SSN && p.SSN <= 100 {
		ssn = 1
	} else {
		ssn = 2
	}

	// The neighbors
	ll := p.foF2var[p.season][hourL][latL][ssn][decile]
	lr := p.foF2var[p.season][hourU][latL][ssn][decile]
	ul := p.foF2var[p.season][hourL][latU][ssn][decile]
	ur := p.foF2var[p.season][hourU][latU][ssn][decile]

	return bilinearInterpolation(ll, lr, ul, ur, r, c)
}
