package hf

import "math"

// eLayerScreeningFrequency calculates the E layer screening frequency of the
// F2 modes (P.533 section 4) and stores the reflection height and the
// frequency in the modes.
func eLayerScreeningFrequency(p *pathData) {
	var dh, deltaf, foE, i float64

	// E layer screening is restricted to paths no longer than 4000 km.
	if p.distance > 4000 {
		return
	}

	// The foE for this calculation.
	if p.distance <= 2000.0 {
		foE = p.CP[mp].foE
	} else {
		foE = cMax(p.CP[t1k].foE, p.CP[r1k].foE)
	}

	// The screening for the F2 modes that exist. The hop count is k + 1.
	for k := p.n0_F2; k < maxF2Mds; k++ {
		dh = p.distance / float64(k+1)

		if p.distance <= p.dmax {
			p.Md_F2[k].hr = mirrorReflectionHeight(p, &p.CP[mp], dh)
		} else if p.distance > p.dmax {
			// The mean of the mirror reflection heights of the control points.
			p.Md_F2[k].hr = (mirrorReflectionHeight(p, &p.CP[td02], dh) +
				mirrorReflectionHeight(p, &p.CP[mp], dh) +
				mirrorReflectionHeight(p, &p.CP[rd02], dh)) / 3.0
		}

		// The elevation angle from equation 13 of section 5.1.
		deltaf = elevationAngle(dh, p.Md_F2[k].hr)

		// The angle of incidence at the height hr = 110 km.
		i = incidenceAngle(deltaf, 110.0)

		// The E layer screening frequency.
		if p.distance <= 2000 {
			p.Md_F2[k].fs = (1.05 * foE) / math.Cos(i)
		} else {
			// The larger foE of the control points 1000 km from the ends.
			p.Md_F2[k].fs = (1.05 * foE) / math.Cos(i)
		}
	}
}

// mirrorReflectionHeight calculates the mirror reflection height by the method
// of P.533 section 5.1 "Elevation angle" for a hop length d.
func mirrorReflectionHeight(p *pathData, cp *controlPt, d float64) float64 {
	hr := 0.0

	var a, b, h, x, xr, y float64
	var hh, z, j, u, g float64
	var a1, a2, b1, b2, e1, e2, f1, f2 float64
	var deltaM float64
	var ds, df float64

	// The critical frequency ratio.
	x = cp.foF2 / cp.foE

	y = cMax(x, 1.8)

	deltaM = (0.18 / (y - 1.4)) + (0.096 * (float64(cMinInt(p.SSN, 160)) - 25.0) / (150.0))

	xr = p.frequency / cp.foF2

	hh = (1490.0 / (cp.M3kF2 + deltaM)) - 316.0

	if x > 3.33 && xr >= 1.0 { // a)
		e1 = -0.09707*math.Pow(xr, 3) + 0.6870*xr*xr - 0.7506*xr + 0.6

		if xr <= 1.71 {
			f1 = -1.862*math.Pow(xr, 4) + 12.95*math.Pow(xr, 3) - 32.03*xr*xr + 33.50*xr - 10.91
		} else {
			f1 = 1.21 + 0.2*xr
		}

		if xr <= 3.7 {
			g = -2.102*math.Pow(xr, 4) + 19.50*math.Pow(xr, 3) - 63.15*xr*xr - 44.73
		} else {
			g = 19.25
		}

		ds = 160.0 + (hh+43.0)*g
		a = (d - ds) / (hh + 140.0)
		a1 = 140.0 + (hh-47.0)*e1
		b1 = 150.0 + (hh-17.0)*f1 - a1

		if b1 >= 0.0 && a >= 0.0 {
			h = a1 + b1*math.Pow(2.4, -a)
		} else {
			h = a1 + b1
		}

		hr = cMin(h, 800.0)
	} else if x > 3.33 && xr < 1.0 { // b)
		z = cMax(xr, 0.1)
		e2 = 0.1906*z*z + 0.00583*z + 0.1936
		a2 = 151.0 + (hh-47.0)*e2
		f2 = 0.645*z*z + 0.883*z + 0.162
		b2 = 141.0 + (hh-24.0)*f2 - a2
		df = cMin(0.115*d/(z*(hh+140.0)), 0.65)
		b = -7.535*math.Pow(df, 4) + 15.75*math.Pow(df, 3) - 8.834*df*df - 0.378*df + 1.0

		if b2 >= 0.0 {
			h = a2 + b2*b
		} else {
			h = a2 + b2
		}

		hr = cMin(h, 800.0)
	} else if x <= 3.33 { // c)
		j = -0.7126*math.Pow(y, 3) + 5.863*y*y - 16.13*y + 16.07
		u = 8.0e-5*(hh-80.0)*(1.0+11.0*math.Pow(y, -2.2)) + 1.2e-3*hh*math.Pow(y, -3.6)
		hr = cMin((115.0 + hh*j + u*d), 800.0)
	}

	return hr
}

// elevationAngle determines the elevation angle from equation (13) of P.533
// for the hop distance dh and the mirror reflection height hr.
func elevationAngle(dh, hr float64) float64 {
	ele := ((1.0 / math.Tan(dh/(2.0*r0))) - ((r0 / (r0 + hr)) / math.Sin((dh / (2.0 * r0)))))
	ele = math.Atan(ele)

	return ele
}

// incidenceAngle determines the angle of incidence from equation (12) of
// P.533 for the elevation angle deltaf and the mirror reflection height hr.
func incidenceAngle(deltaf, hr float64) float64 {
	return math.Asin(r0 * math.Cos(deltaf) / (r0 + hr))
}
