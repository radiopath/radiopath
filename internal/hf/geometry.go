package hf

import "math"

// greatCircleDistance returns the great circle distance in km between here
// and there.
func greatCircleDistance(here, there location) float64 {
	return 2.0 * r0 * math.Asin(math.Sqrt(math.Pow(math.Sin((here.lat-there.lat)/2.0), 2.0)+math.Cos(here.lat)*math.Cos(there.lat)*math.Pow(math.Sin((here.lng-there.lng)/2.0), 2.0)))
}

// greatCirclePoint finds the point at fraction*distance from here on the great
// circle to there and stores its location and distance in midpnt.
func greatCirclePoint(here, there location, midpnt *controlPt, distance, fraction float64) {
	if distance != 0.0 {
		midpnt.distance = distance * fraction
		d := distance / r0
		a := math.Sin((1-fraction)*d) / math.Sin(d)
		b := math.Sin(fraction*d) / math.Sin(d)
		x := a*math.Cos(here.lat)*math.Cos(here.lng) + b*math.Cos(there.lat)*math.Cos(there.lng)
		y := a*math.Cos(here.lat)*math.Sin(here.lng) + b*math.Cos(there.lat)*math.Sin(there.lng)
		z := a*math.Sin(here.lat) + b*math.Sin(there.lat)
		midpnt.L.lat = math.Atan2(z, math.Sqrt(math.Pow(x, 2)+math.Pow(y, 2)))
		midpnt.L.lng = math.Atan2(y, x)
	} else if distance == 0 {
		midpnt.distance = 0.0
		midpnt.L.lat = here.lat
		midpnt.L.lng = here.lng
	}
}

// geomagneticCoords converts geographic coordinates to geomagnetic ones.
func geomagneticCoords(here location) location {
	// Pole location of 1955 when the coefficients were determined.
	var pole location
	pole.lat = 78.5 * d2r
	pole.lng = -68.2 * d2r

	var there location
	there.lat = math.Asin(math.Sin(here.lat)*math.Sin(pole.lat) + math.Cos(here.lat)*math.Cos(pole.lat)*math.Cos(here.lng-pole.lng))
	there.lng = math.Asin(math.Cos(here.lat) * math.Sin(here.lng-pole.lng) / math.Cos(there.lat))
	return there
}

// bearing returns the bearing in radians from here to there, N and E positive;
// for a long path the opposite direction is returned.
func bearing(here, there location, direction int) float64 {
	numerator := math.Sin(there.lng-here.lng) * math.Cos(there.lat)
	denominator := math.Cos(here.lat)*math.Sin(there.lat) - math.Sin(here.lat)*math.Cos(there.lat)*math.Cos(there.lng-here.lng)

	b := math.Atan2(numerator, denominator)

	b = cFmod(2.0*pi+b, 2.0*pi)

	if direction == longPath {
		// Flip the bearing around for the long way round.
		b = cFmod(2.0*pi+(b+pi), 2.0*pi)
	}

	return b
}
