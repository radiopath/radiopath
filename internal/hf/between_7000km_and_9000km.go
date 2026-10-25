package hf

import "math"

// between7000kmand9000km interpolates the field strength between the path
// distances of 7000 and 9000 km (P.533 section 5.4) and sets the basic MUF of
// the path.
func between7000kmand9000km(p *pathData) {
	var xs, xl, xi float64
	var b, dmax float64
	var bmuf [2]float64

	if 7000.0 < p.distance && p.distance < 9000.0 {
		xl = math.Pow(10.0, p.El/100.0)
		xs = math.Pow(10.0, p.Es/100.0)

		xi = xs + ((p.distance-7000.0)/2000.0)*(xl-xs)

		p.Ei = 100.0 * math.Log10(xi) // equation (42)

		// The basic MUF according to section 5.4.
		b = calcB(&p.CP[td02])
		dmax = cMin(calcdmax(&p.CP[td02]), 4000.0)
		bmuf[0] = calcF2DMUF(&p.CP[td02], p.distance/float64(p.n0_F2+1), dmax, b)

		b = calcB(&p.CP[rd02])
		dmax = cMin(calcdmax(&p.CP[rd02]), 4000.0)
		bmuf[1] = calcF2DMUF(&p.CP[rd02], p.distance/float64(p.n0_F2+1), dmax, b)

		p.BMUF = cMin(bmuf[0], bmuf[1])
	}
}
