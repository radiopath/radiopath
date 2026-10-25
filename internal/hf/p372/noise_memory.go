package p372

// Coefficients holds the monthly atmospheric noise coefficients that the C
// code keeps in NoiseParams (fakp, fakabp, dud, fam). Allocation is done by
// the Go runtime, so there is no counterpart of AllocateNoiseMemory or
// FreeNoiseMemory. A Coefficients value is read-only after loading and may be
// shared between goroutines.
type Coefficients struct {
	// Fakp is the Fourier series of Fam at 1 MHz, indexed [timeblock][k][j].
	Fakp [6][16][29]float64
	// Fakabp is the linear normalisation of Fakp, indexed [timeblock][0..1].
	Fakabp [6][2]float64
	// Dud holds the decile and sigma polynomials, indexed [quantity][block][k].
	Dud [5][12][5]float64
	// Fam holds the frequency variation polynomials, indexed [block][k].
	Fam [12][14]float64
}
