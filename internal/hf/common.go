package hf

import "math"

// The constants of Common.h. They are variables, not constants, on purpose:
// Go evaluates constant expressions exactly while C rounds every operation to
// double, and the port has to reproduce the C rounding.
var (
	pi   = 3.14159265358979323846
	r0   = 6371.009     // IUGG mean Earth radius (km)
	d2r  = 0.0174532925 // Truncated PI/180, as in the C code
	r2d  = 57.2957795   // Truncated 180/PI, as in the C code
	vOfL = 299792458.0  // Velocity of light (m/s)

	dblEpsilon = 2.2204460492503131e-016
	tinyDB     = -307.0 // DBL_MIN_10_EXP: smallest number in dB
	tooBig     = math.MaxFloat64
)

// cMin is the C macro min(x, y) which returns y when the comparison is false
// (relevant for NaN).
func cMin(x, y float64) float64 {
	if x < y {
		return x
	}
	return y
}

// cMax is the C macro max(x, y) which returns y when the comparison is false
// (relevant for NaN).
func cMax(x, y float64) float64 {
	if x > y {
		return x
	}
	return y
}

// cMinInt is the C macro min(x, y) for integers.
func cMinInt(x, y int) int {
	if x < y {
		return x
	}
	return y
}

// cMaxInt is the C macro max(x, y) for integers.
func cMaxInt(x, y int) int {
	if x > y {
		return x
	}
	return y
}

// cFmod is the C fmod(); Go's math.Mod has the same semantics.
func cFmod(x, y float64) float64 {
	return math.Mod(x, y)
}
