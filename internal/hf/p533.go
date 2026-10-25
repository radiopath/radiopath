// Package hf is a Go port of the ITU-R P.533-14 HF circuit performance model
// (MUF, field strength, noise, SNR and circuit reliability) with the P.372
// noise model in the sub-package p372. The core keeps the structure of the
// ITU C reference code (PathData, five control points, E and F2 modes) so that
// it stays comparable to it line by line.
package hf

import "github.com/radiopath/radiopath/internal/hf/p372"

// Indices of the ionospheric maps. The gyrofrequency and magnetic dip are
// calculated at 100 km (absorption) or 300 km (everything else).
const (
	hr100km = 0
	hr300km = 1
)

// Control point indices. They are named after the short model; the long model
// uses them with another meaning (diagnostics only).
const (
	t1k  = 0 // T + 1000 km
	td02 = 1 // T + d0/2
	mp   = 2 // Mid-path
	rd02 = 3 // R - d0/2
	r1k  = 4 // R - 1000 km
)

// Seasons, index of the foF2 variability array.
const (
	winter  = 0
	equinox = 1
	summer  = 2
)

// Decile flags.
const (
	dl = 0 // Lower decile
	du = 1 // Upper decile
)

// Day and night index of the Rop array in MUFOperational().
const (
	day   = 0
	night = 1
)

// Zero-based months.
const (
	jan = iota
	feb
	mar
	apr
	may
	jun
	jul
	aug
	sep
	oct
	nov
	dec
)

// Indicators for "no lowest order mode" and "no dominant mode".
const (
	noLowestMode   = 99
	noDominantMode = 99
)

// Modulation flags.
const (
	analog  = 0
	digital = 1
)

// Short or long path flags.
const (
	shortPath = 0
	longPath  = 1
)

const (
	minEleAngleS = 3.0 // Minimum elevation angle for the short model (degrees)
	minEleAngleL = 3.0 // Minimum elevation angle for the long model (degrees)
	maxSSN       = 160 // Maximum sunspot number
	maxF2Mds     = 6   // Maximum number of F2 modes
	maxEMds      = 3   // Maximum number of E modes
	maxMds       = maxEMds + maxF2Mds
)

// Direction of the antenna gain calculation.
const (
	txToRx = 1
	rxToTx = 2
)

// ionosphere map dimensions, fixed by the iongrid program that wrote the maps.
const (
	ionHours = 24
	ionLng   = 241
	ionLat   = 121
	ionSSN   = 2
)

// location is a point on the Earth in radians (north and east positive).
type location struct {
	lat, lng float64
}

// solarParameters holds the sun position at a control point.
type solarParameters struct {
	ha   float64 // hour angle (radians)
	sha  float64 // sunrise/sunset hour angle (radians)
	sza  float64 // solar zenith angle (radians)
	decl float64 // solar declination (radians)
	eot  float64 // equation of time (minutes)
	lsr  float64 // local sunrise (hours)
	lsn  float64 // local solar noon (hours)
	lss  float64 // local sunset (hours)
}

// controlPt is a control point of the path (struct ControlPt).
type controlPt struct {
	L        location
	distance float64    // distance from the transmitter to the point (km)
	foE      float64    // E layer critical frequency (MHz)
	foF2     float64    // F2 layer critical frequency (MHz)
	M3kF2    float64    // M(3000)F2
	dip      [2]float64 // magnetic dip (radians)
	fH       [2]float64 // gyrofrequency (MHz)
	ltime    float64    // local time (hours)
	hr       float64    // mirror reflection height (km)
	x        float64    // foE/foF2 ratio used in the F2 MUF
	Sun      solarParameters
}

// mode is one E or F2 propagation mode (struct Mode).
type mode struct {
	BMUF    float64 // basic MUF; non-zero means that the mode exists
	MUF90   float64 // MUF exceeded for 90% of the days
	MUF50   float64 // MUF exceeded for 50% of the days
	MUF10   float64 // MUF exceeded for 10% of the days
	OPMUF   float64 // operational MUF
	OPMUF10 float64
	OPMUF90 float64
	Fprob   float64 // probability that the mode is supported at the frequency
	deltal  float64 // lower decile of the MUF
	deltau  float64 // upper decile of the MUF
	hr      float64 // reflection height
	fs      float64 // E layer screening frequency (F2 modes only)
	Lb      float64 // basic loss
	Ew      float64 // field strength (dB(1 uV/m))
	ele     float64 // elevation angle
	Prw     float64 // receiver power (dBW)
	Grw     float64 // receive antenna gain (dBi)
	tau     float64 // time delay
	MC      int
}

// antenna holds an antenna pattern [freq][azimuth][elevation] (struct Antenna).
type antenna struct {
	name    string
	freqn   int           // number of frequencies with pattern data
	freqs   []float64     // the frequencies
	pattern [][][]float64 // gain in dBi; nil when not loaded
}

// pathData is the whole state of one P.533 calculation (struct PathData).
type pathData struct {
	// User provided input
	year  int
	month int // 0-11
	hour  int // hour index 0-23 (UTC hour minus one)
	SSN   int // 12-month smoothed sunspot number

	Modulation int
	SorL       int

	frequency float64 // MHz
	BW        float64 // Hz
	txpower   float64 // dB(1 kW)

	SNRXXp int     // required reliability (%) of the signal-to-noise ratio
	SNRr   float64 // required signal-to-noise ratio (dB)
	SIRr   float64 // required signal-to-interference ratio (dB)

	F0 float64 // frequency dispersion at -10 dB (digital)
	T0 float64 // time spread at -10 dB (digital)

	A  float64 // required A ratio (dB)
	TW float64 // time window (ms)
	FW float64 // frequency window (Hz)

	L_tx, L_rx location
	A_tx, A_rx antenna

	// Data arrays, loaded by the caller.
	foF2    *ionMap
	M3kF2   *ionMap
	foF2var *decileFactors

	// Calculated parameters
	season   int
	distance float64
	ptick    float64
	dmax     float64
	B        float64
	ele      float64

	BMUF    float64
	MUF50   float64
	MUF90   float64
	MUF10   float64
	OPMUF   float64
	OPMUF90 float64
	OPMUF10 float64

	n0_F2 int
	n0_E  int

	Es float64
	El float64
	Ei float64
	Ep float64
	Pr float64

	Lz float64

	E0  float64
	Gap float64
	Ly  float64
	fM  float64
	fL  float64
	F   float64
	fH  float64
	Gtl float64
	K   [2]float64

	SNR  float64
	DuSN float64
	DlSN float64

	SNRXX float64

	SIR  float64
	DuSI float64
	DlSI float64
	RSN  float64
	RT   float64
	RF   float64

	BCR     float64
	OCR     float64
	OCRs    float64
	MIR     float64
	probocc float64

	Grw  float64
	EIRP float64

	CP [5]controlPt

	Md_F2 [maxF2Mds]mode
	Md_E  [maxEMds]mode

	DMptr *mode
	DMidx int

	// noiseP is the P.372 noise state; coeff the atmospheric noise
	// coefficients of the month (the dud and fam arrays of the C struct).
	noiseP p372.Noise
	coeff  *p372.Coefficients
}
