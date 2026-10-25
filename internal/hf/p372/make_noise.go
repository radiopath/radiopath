package p372

// MakeNoise is the stand-alone caller of the P.372 method (MakeNoise() in the
// C code, without the printing options). month is 0-based, hour as in
// Noise.Hour, lat and lng are in decimal degrees and freq is in MHz. The
// result holds FaA, DuA, DlA, FaM, DuM, DlM, FaG, DuG, DlG, FamT, DuT, DlT.
func MakeNoise(month, hour int, lat, lng, freq, mmnoise float64, dir string) ([12]float64, error) {
	var out [12]float64

	c, err := LoadCoefficients(dir, month)
	if err != nil {
		return out, err
	}

	n := Noise{
		Hour:         hour,
		Lng:          lng * d2r,
		Lat:          lat * d2r,
		Frequency:    freq,
		ManMadeNoise: mmnoise,
	}
	n.Initialize()
	if err := Compute(&n, c); err != nil {
		return out, err
	}

	out = [12]float64{n.FaA, n.DuA, n.DlA, n.FaM, n.DuM, n.DlM, n.FaG, n.DuG, n.DlG, n.FamT, n.DuT, n.DlT}
	return out, nil
}
