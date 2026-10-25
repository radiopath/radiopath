package p372

import "errors"

// Errors returned by this package. They replace the RTN_ERR* codes of Noise.h.
var (
	// ErrOpenCoeffFile corresponds to RTN_ERROPENCOEFFFILE.
	ErrOpenCoeffFile = errors.New("p372: cannot open coefficient file")
	// ErrBadCoeffFile is returned when a coefficient file is truncated or malformed.
	ErrBadCoeffFile = errors.New("p372: malformed coefficient file")
	// ErrMonth is returned when the month index is outside 0..11.
	ErrMonth = errors.New("p372: month must be in 0..11")
	// ErrNoCoefficients is returned when Compute is called without coefficients.
	ErrNoCoefficients = errors.New("p372: coefficients are missing")
)
