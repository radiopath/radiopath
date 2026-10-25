package hf

import "math"

// calculateCPParameters finds the ionospheric parameters foF2, M3kF2 and foE,
// the gyrofrequency and magnetic dip at 100 and 300 km and the solar
// parameters of the control point here.
func calculateCPParameters(p *pathData, here *controlPt) {
	// foF2 and M3kF2, bilinear interpolation when not on a grid point.
	ionosphericParameters(here, p.foF2, p.M3kF2, p.hour, p.SSN)

	// The solar parameters are needed for the long model and for foE.
	solarParams(here, p.month, float64(p.hour))

	// foE by the method of P.1239-2.
	findfoE(here, p.month, p.hour, p.SSN)

	// 300 km for the MUF and long path, 100 km for the absorption.
	magfit(here, 100.00)
	magfit(here, 300.00)
}

// neighbor is a grid point around the point of interest.
type neighbor struct {
	foF2, M3kF2 [2]float64 // the values at SSN 0 and 100
	j, k        int        // longitude and latitude index into the map
}

// ionosphericParameters finds foF2 and M(3000)F2 by bilinear interpolation of
// the maps and interpolates linearly in the sunspot number.
func ionosphericParameters(here *controlPt, foF2, m3kF2 *ionMap, hour, ssn int) {
	var ul, ur, lr, ll, ned neighbor

	const (
		zerolat = 60  // The max latitude index is 120
		zerolng = 120 // The max longitude index is 240
		lng     = ionLng
		lat     = ionLat
	)

	// The routine depends on the 1.5 degree increment of the maps.
	inc := 1.5 * d2r

	// The neighbors are ordered per quadrant so that they line up with the
	// array indices; (0,0) of the maps is the south west corner.
	if here.L.lat >= 0.0 {
		if here.L.lng >= 0.0 { // NE quadrant
			ll.k = zerolat + int(here.L.lat/inc)
			ll.j = zerolng + int(here.L.lng/inc)
			lr.k = ll.k
			lr.j = ll.j + 1
			ur.k = ll.k + 1
			ur.j = ll.j + 1
			ul.k = ll.k + 1
			ul.j = ll.j
			if ll.j != lng-1 { // not on E edge
				if ll.k != lat-1 { // not on N edge
					// nothing to do
				} else { // N edge, no rollover N to S
					ur.k = ll.k
					ul.k = ll.k
				}
			} else { // E edge
				if ll.k != lat-1 { // not on N edge, rollover E to W
					lr.j = 0
					ur.j = 0
				} else { // NE corner, collapse to 1D
					lr.k = ll.k
					lr.j = 0
					ur.k = ll.k
					ur.j = 0
					ul.k = ll.k
					ul.j = ll.j
				}
			}
		} else { // NW quadrant
			lr.k = zerolat + int(here.L.lat/inc)
			lr.j = zerolng + int(here.L.lng/inc)
			ll.k = lr.k
			ll.j = lr.j - 1
			ul.k = lr.k + 1
			ul.j = lr.j - 1
			ur.k = lr.k + 1
			ur.j = lr.j
			if lr.j != 0 { // not on W edge
				if lr.k != lat-1 { // not on N edge
					// nothing to do
				} else { // N edge, no rollover N to S
					ur.k = lr.k
					ul.k = lr.k
				}
			} else { // W edge
				if lr.k != lat-1 { // not on N edge, rollover W to E
					ll.j = lng - 1
					ul.j = lng - 1
				} else { // NW corner, collapse to 1D
					ll.k = lr.k
					ll.j = lng - 1
					ur.k = lr.k
					ur.j = lr.j
					ul.k = lr.k
					ul.j = lng - 1
				}
			}
		}
	} else {
		if here.L.lng >= 0.0 { // SE quadrant
			ul.k = zerolat + int(here.L.lat/inc)
			ul.j = zerolng + int(here.L.lng/inc)
			ur.k = ul.k
			ur.j = ul.j + 1
			ll.k = ul.k - 1
			ll.j = ul.j
			lr.k = ul.k - 1
			lr.j = ul.j + 1
			if ul.j != lng-1 { // not on E edge
				if ul.k != 0 { // not on S edge
					// nothing to do
				} else { // S edge, no rollover S to N
					ll.k = ul.k
					lr.k = ul.k
				}
			} else { // E edge
				if ul.k != 0 { // rollover E to W
					lr.j = 0
					ur.j = 0
				} else { // SE corner, collapse to 1D
					lr.k = ul.k
					lr.j = 0
					ur.k = ul.k
					ur.j = 0
					ll.k = ul.k
					ll.j = ul.j
				}
			}
		} else { // SW quadrant
			ur.k = zerolat + int(here.L.lat/inc)
			ur.j = zerolng + int(here.L.lng/inc)
			ul.k = ur.k
			ul.j = ur.j - 1
			ll.k = ur.k - 1
			ll.j = ur.j - 1
			lr.k = ur.k - 1
			lr.j = ur.j
			if ur.j != 0 { // not on W edge
				if ur.k != 0 { // not on S edge
					// nothing to do
				} else { // S edge, no rollover S to N
					lr.k = ur.k
					ll.k = ur.k
				}
			} else { // W edge
				if ur.k != 0 { // rollover W to E
					ll.j = lng - 1
					ul.j = lng - 1
				} else { // SW corner, collapse to 1D
					lr.k = ur.k
					lr.j = ur.j
					ll.k = ur.k
					ll.j = lng - 1
					ul.k = ur.k
					ul.j = lng - 1
				}
			}
		}
	}

	// Populate foF2 and M3kF2 of the four neighbors.
	n := hour
	for m := 0; m < 2; m++ { // SSN
		ul.foF2[m] = foF2.at(n, ul.j, ul.k, m)
		ul.M3kF2[m] = m3kF2.at(n, ul.j, ul.k, m)
		ur.foF2[m] = foF2.at(n, ur.j, ur.k, m)
		ur.M3kF2[m] = m3kF2.at(n, ur.j, ur.k, m)
		ll.foF2[m] = foF2.at(n, ll.j, ll.k, m)
		ll.M3kF2[m] = m3kF2.at(n, ll.j, ll.k, m)
		lr.foF2[m] = foF2.at(n, lr.j, lr.k, m)
		lr.M3kF2[m] = m3kF2.at(n, lr.j, lr.k, m)
	}

	// Fractional row and column for the bilinear interpolation.
	frack := math.Abs(here.L.lat/inc) - float64(int(math.Abs(here.L.lat/inc)))
	fracj := math.Abs(here.L.lng/inc) - float64(int(math.Abs(here.L.lng/inc)))
	for m := 0; m < 2; m++ {
		ned.foF2[m] = bilinearInterpolation(ll.foF2[m], lr.foF2[m], ul.foF2[m], ur.foF2[m], frack, fracj)
		ned.M3kF2[m] = bilinearInterpolation(ll.M3kF2[m], lr.M3kF2[m], ul.M3kF2[m], ur.M3kF2[m], frack, fracj)
	}

	// Linear interpolation in the SSN (0 and 100), restricted to 160.
	ssn = cMinInt(ssn, maxSSN)
	fssn := float64(ssn)
	here.foF2 = (ned.foF2[1]*fssn + ned.foF2[0]*(100.0-fssn)) / 100.0
	here.M3kF2 = (ned.M3kF2[1]*fssn + ned.M3kF2[0]*(100.0-fssn)) / 100.0
}

// findfoE determines the critical frequency of the E layer by the method of
// ITU-R P.1239; the solar parameters of the control point must be known.
func findfoE(here *controlPt, month, hour, ssn int) {
	var a, b, phi, m, n, c, x, y, d, dsza, pp, h float64

	ssn = cMinInt(ssn, maxSSN)
	fssn := float64(ssn)

	// A: solar activity factor, phi is the 12-month 10.7 cm flux (eq 2).
	phi = 63.7 + 0.728*fssn + 0.00089*math.Pow(fssn, 2)
	a = 1.0 + 0.0094*(phi-66.0)

	// B: seasonal factor
	if math.Abs(here.L.lat) < 32.0*d2r {
		m = -1.93 + 1.92*math.Cos(here.L.lat)
	} else {
		m = 0.11 - 0.49*math.Cos(here.L.lat)
	}

	if math.Abs(here.L.lat-here.Sun.decl) < 80.0*d2r {
		n = here.L.lat - here.Sun.decl
	} else {
		n = 80.0 * d2r
	}

	b = math.Pow(math.Cos(n), m)

	// C: main latitude factor
	if math.Abs(here.L.lat) < 32.0*d2r {
		x = 23.0
		y = 116.0
	} else {
		x = 92.0
		y = 35.0
	}
	c = x + y*math.Cos(here.L.lat)

	// D: time-of-day factor
	if math.Abs(here.L.lat) <= 12.0*d2r {
		pp = 1.31
	} else {
		pp = 1.2
	}

	if here.Sun.sza <= 73.0*d2r {
		d = math.Pow(math.Cos(here.Sun.sza), pp)
	} else if here.Sun.sza > 73.0*d2r && here.Sun.sza < pi/2.0 { // twilight
		dsza = 6.27e-13 * math.Pow(here.Sun.sza*r2d-50.0, 8.0) * d2r
		d = math.Pow(math.Cos(here.Sun.sza-dsza), pp)
	} else {
		// Night: the local sunset and sunrise tell the hours after sunset.
		hour = (hour + 1) % 24 // adjust time and roll over
		fhour := float64(hour)
		if here.Sun.lss >= here.Sun.lsr && fhour >= here.Sun.lss && fhour >= here.Sun.lsr {
			h = fhour - here.Sun.lss
		} else if here.Sun.lss < here.Sun.lsr && fhour >= here.Sun.lss && fhour < here.Sun.lsr {
			h = fhour - here.Sun.lss
		} else if here.Sun.lss >= here.Sun.lsr && fhour < here.Sun.lss && fhour < here.Sun.lsr {
			h = 24.0 - here.Sun.lss + fhour
		} else {
			h = 0.0
		}

		// Polar winter (civil twilight at 72.5622 degrees is used).
		if (here.L.lat > 72.5622*d2r && (month == nov || month == dec || month == jan)) ||
			(here.L.lat < 72.5622*d2r && (month == may || month == jun || month == jul)) {
			d = math.Pow(0.072, pp) * math.Exp(25.2-0.28*here.Sun.sza*r2d)
		} else {
			// The larger of the two calculations
			d = cMax(math.Pow(0.072, pp)*math.Exp(-1.4*h), math.Pow(0.072, pp)*math.Exp(25.2-0.28*here.Sun.sza*r2d))
		}
	}

	// The larger of the foE calculations
	here.foE = cMax(math.Pow(a*b*c*d, 0.25), math.Pow(0.004*math.Pow(1.0+0.021*phi, 2), 0.25))
}

// solarParams determines the solar parameters at a control point for the 15th
// day of the month and the UTC hour.
func solarParams(here *controlPt, month int, hour float64) {
	a := 0.98565327            // average angle per day
	b := 3.98891967            // minutes per degree of Earth's rotation
	s := math.Sin(23.45 * d2r) // Earth's tilt sine
	c := math.Cos(23.45 * d2r) // Earth's tilt cosine
	v := 78.746118 * d2r       // value of nu on March 21st
	doty := [12]int{0, 31, 59, 90, 120, 152, 181, 212, 243, 273, 304, 334}

	// Local time and time zone, truncated to whole hours as in C.
	ltime := hour + float64(int(here.L.lng/(15.0*d2r)))
	tzone := float64(int(here.L.lng / (15.0 * d2r)))

	// Only the 15th day of the month is supported.
	dayOfMonth := 15

	dd := float64(doty[month]+dayOfMonth) + hour/24.0

	// Equation of time. In dd days the Earth moves through lambda.
	lambda := a * d2r * (dd - 2)

	// Arc length due to the elliptical orbit.
	nu := lambda + 1.915169*d2r*math.Sin(lambda)

	// Mean sun angle after N - 80 days.
	epsilon := a * d2r * (dd - 80)

	// epsilon is +- PI/2
	if epsilon >= 270*d2r {
		epsilon -= 2.0 * pi
	} else if epsilon >= 90*d2r {
		epsilon -= pi
	}

	// The angle of the true sun.
	beta := math.Atan(c * math.Tan(epsilon))

	// Equation of time = tilt effect + elliptic effect.
	here.Sun.eot = b * ((epsilon - beta) + (lambda - nu)) * r2d

	// Solar declination in radians.
	here.Sun.decl = math.Asin(s * math.Sin((math.Sin(a*(dd-2)*d2r)*0.016713+a*(dd-2)*d2r)-v))

	// Hour angle from the solar time corrected for longitude and eot.
	toffset := ((here.L.lng/(15.0*d2r))-tzone)*60.0 + here.Sun.eot // minutes

	tst := ltime*60 + toffset // true solar time in minutes

	here.Sun.ha = ((tst / 4.0) - 180) * d2r

	// Hour angle at sunrise and sunset.
	here.Sun.sha = math.Acos((math.Cos(90.833*d2r) / (math.Cos(here.L.lat) * math.Cos(here.Sun.decl))) - (math.Tan(here.L.lat) * math.Tan(here.Sun.decl)))

	// Cosine of the solar zenith angle.
	cosphi := (math.Sin(here.L.lat) * math.Sin(here.Sun.decl)) + (math.Cos(here.L.lat) * math.Cos(here.Sun.decl) * math.Cos(here.Sun.ha))

	// Watch out for roundoff errors.
	if math.Abs(cosphi) > 1.0 {
		if cosphi >= 0.0 {
			cosphi = 1.0
		} else {
			cosphi = -1.0
		}
	}

	here.Sun.sza = math.Acos(cosphi) // positive even in the southern hemisphere

	// Local sunrise, sunset and noon relative to UTC in fractional hours.
	here.Sun.lsr = (720.0 + (-here.L.lng-here.Sun.sha)*r2d*4.0 - here.Sun.eot) / 60.0
	here.Sun.lss = (720.0 + (-here.L.lng+here.Sun.sha)*r2d*4.0 - here.Sun.eot) / 60.0
	here.Sun.lsn = (720.0 + (-here.L.lng)*r2d*4.0 - here.Sun.eot) / 60.0

	// Roll over the times; 24 is added because fmod(x, 24) may get negative x.
	here.Sun.lsr = cFmod(here.Sun.lsr+24.0, 24.0)
	here.Sun.lss = cFmod(here.Sun.lss+24.0, 24.0)
	here.Sun.lsn = cFmod(here.Sun.lsn+24.0, 24.0)

	// Store the UTC time.
	here.ltime = hour
}

// bilinearInterpolation interpolates a value from its four neighbors by the
// method of ITU-R P.1144; r is the fractional row and c the fractional column.
func bilinearInterpolation(ll, lr, ul, ur, r, c float64) float64 {
	return ll*((1.0-r)*(1.0-c)) +
		ul*((r)*(1.0-c)) +
		lr*((1.0-r)*(c)) +
		ur*((r)*(c))
}
