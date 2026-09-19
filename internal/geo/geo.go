package geo

import "math"

const EarthRadius = 6371008.8 // IUGG mean radius, meters

type Point struct {
	Lat, Lon float64
}

func rad(deg float64) float64 { return deg * math.Pi / 180 }
func deg(rad float64) float64 { return rad * 180 / math.Pi }

func Distance(a, b Point) float64 {
	lat1, lat2 := rad(a.Lat), rad(b.Lat)
	dlat := lat2 - lat1
	dlon := rad(b.Lon - a.Lon)
	h := math.Sin(dlat/2)*math.Sin(dlat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dlon/2)*math.Sin(dlon/2)
	return 2 * EarthRadius * math.Asin(math.Min(1, math.Sqrt(h)))
}

func InitialBearing(a, b Point) float64 {
	lat1, lat2 := rad(a.Lat), rad(b.Lat)
	dlon := rad(b.Lon - a.Lon)
	y := math.Sin(dlon) * math.Cos(lat2)
	x := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(dlon)
	return math.Mod(deg(math.Atan2(y, x))+360, 360)
}

func PointAt(a, b Point, frac float64) Point {
	if frac <= 0 {
		return a
	}
	if frac >= 1 {
		return b
	}
	d := Distance(a, b) / EarthRadius
	if d == 0 {
		return a
	}
	lat1, lon1 := rad(a.Lat), rad(a.Lon)
	lat2, lon2 := rad(b.Lat), rad(b.Lon)
	A := math.Sin((1-frac)*d) / math.Sin(d)
	B := math.Sin(frac*d) / math.Sin(d)
	x := A*math.Cos(lat1)*math.Cos(lon1) + B*math.Cos(lat2)*math.Cos(lon2)
	y := A*math.Cos(lat1)*math.Sin(lon1) + B*math.Cos(lat2)*math.Sin(lon2)
	z := A*math.Sin(lat1) + B*math.Sin(lat2)
	return Point{
		Lat: deg(math.Atan2(z, math.Sqrt(x*x+y*y))),
		Lon: deg(math.Atan2(y, x)),
	}
}

func Destination(a Point, bearingDeg, distM float64) Point {
	lat1, lon1 := rad(a.Lat), rad(a.Lon)
	brg := rad(bearingDeg)
	d := distM / EarthRadius
	lat2 := math.Asin(math.Sin(lat1)*math.Cos(d) + math.Cos(lat1)*math.Sin(d)*math.Cos(brg))
	lon2 := lon1 + math.Atan2(math.Sin(brg)*math.Sin(d)*math.Cos(lat1), math.Cos(d)-math.Sin(lat1)*math.Sin(lat2))
	return Point{Lat: deg(lat2), Lon: deg(math.Mod(lon2+3*math.Pi, 2*math.Pi) - math.Pi)}
}
