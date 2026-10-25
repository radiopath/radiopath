package hf

import (
	"fmt"
	"math"
	"strings"
)

// Man-made noise environments for Request.ManMadeNoise (ITU-R P.372). Any
// other positive value is a man-made noise figure Fam in dB at 3 MHz; a
// negative value cancels the noise calculation and uses minus the value as the
// total noise.
const (
	NoiseCity        = 0.0
	NoiseResidential = 1.0
	NoiseRural       = 2.0
	NoiseQuietRural  = 3.0
	NoiseNoisy       = 4.0
	NoiseQuiet       = 5.0
)

// ParseNoiseEnvironment converts the name of a man-made noise environment
// (CITY, RESIDENTIAL, RURAL, QUIETRURAL, NOISY or QUIET, case insensitive)
// to the value for Request.ManMadeNoise.
func ParseNoiseEnvironment(name string) (float64, bool) {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "CITY":
		return NoiseCity, true
	case "RESIDENTIAL":
		return NoiseResidential, true
	case "RURAL":
		return NoiseRural, true
	case "QUIETRURAL":
		return NoiseQuietRural, true
	case "NOISY":
		return NoiseNoisy, true
	case "QUIET":
		return NoiseQuiet, true
	}
	return 0, false
}

// Modulation is the modulation class of the circuit.
type Modulation int

// The modulation classes.
const (
	Analog  Modulation = Modulation(analog)
	Digital Modulation = Modulation(digital)
)

// DigitalParams holds the parameters that only matter for digital modulation.
type DigitalParams struct {
	RequiredSIR float64 // required signal-to-interference ratio (dB)
	F0          float64 // frequency dispersion at -10 dB of the peak (Hz)
	T0          float64 // time spread at -10 dB of the peak (ms)
	A           float64 // required amplitude ratio (dB)
	TW          float64 // time window (ms)
	FW          float64 // frequency window (Hz)
}

// Antenna describes the antenna of one end of the circuit. Without File and
// Data the antenna is isotropic with the gain GainOffset (dBi). Otherwise it
// is a VOACAP antenna file of type 11, 13 or 14 (type 13 is the 360 x 91
// pattern, e.g. derived from NEC); the gain offset is not applied to files, as
// in the C driver.
type Antenna struct {
	File       string   // path of a VOACAP type 11, 13 or 14 file
	Data       []byte   // the content of such a file, used instead of File
	GainOffset float64  // gain of the isotropic antenna (dBi)
	Bearing    *float64 // main beam azimuth for patterns and type 13 (degrees); nil points at the other end

	// Pattern, when set, takes precedence over File and Data: the gain in dBi at an azimuth offset from the main
	// beam (degrees, 0 to 359) and an elevation above the horizon (degrees, 0 to 90).
	Pattern func(azimuthOffset, elevation float64) float64
}

// Request is the input of one prediction: one path in one month, for a list
// of UTC hours and frequencies.
type Request struct {
	TxLat, TxLng float64 // transmitter, degrees (north and east positive)
	RxLat, RxLng float64 // receiver, degrees

	Year  int // 1900 to 2100
	Month int // 1 to 12
	Hours []int
	SSN   int // 12-month smoothed sunspot number

	Frequencies []float64 // MHz, 1 to 30

	TxPower      float64 // dB(1 kW)
	Bandwidth    float64 // Hz
	RequiredSNR  float64 // dB
	SNRXXPercent int     // time percentage of the SNR to report, 1 to 99
	ManMadeNoise float64 // see the Noise constants

	Modulation Modulation
	Digital    DigitalParams

	TxAntenna Antenna
	RxAntenna Antenna

	LongPath bool // use the long way round the globe
}

// validate checks what the C driver (ValidateITURHFP) and the input reader
// check, apart from the checks of ValidatePath() that run per calculation.
func (r *Request) validate() error {
	if len(r.Hours) == 0 {
		return ErrNoHours
	}
	for _, h := range r.Hours {
		if h < 1 || h > 24 {
			return ErrHour
		}
	}
	if len(r.Frequencies) == 0 {
		return ErrNoFrequencies
	}
	for _, f := range r.Frequencies {
		if !(1.0 <= f && f <= 30.0) {
			return ErrFrequency
		}
	}
	if r.Month < 1 || r.Month > 12 {
		return ErrMonth
	}
	for _, a := range []*Antenna{&r.TxAntenna, &r.RxAntenna} {
		if a.GainOffset < tinyDB || a.GainOffset > 60.0 {
			return ErrGainOffset
		}
		if a.Bearing != nil && (*a.Bearing < 0 || *a.Bearing > 360) {
			return ErrBearing
		}
	}
	return nil
}

// antennaPattern builds the pattern of an antenna. Isotropic antennas are
// filled with the gain offset; files are read with the main beam at bearing
// (radians), like ReadAntennaPatterns() of the C driver.
func antennaPattern(a *Antenna, bearing float64) (*antenna, error) {
	if a.Pattern != nil {
		return functionPattern(a.Pattern, bearing)
	}
	if a.File == "" && len(a.Data) == 0 {
		ant := new(antenna)
		isotropicPattern(ant, a.GainOffset)
		return ant, nil
	}
	if len(a.Data) > 0 {
		return parseAntenna(strings.NewReader(string(a.Data)), bearing)
	}
	return readAntennaFile(a.File, bearing)
}

// newPath converts the request to the path structure of the core. The path
// still lacks the month dependent data arrays.
func (r *Request) newPath() (*pathData, error) {
	p := new(pathData)

	p.L_tx.lat = r.TxLat * d2r
	p.L_tx.lng = r.TxLng * d2r
	p.L_rx.lat = r.RxLat * d2r
	p.L_rx.lng = r.RxLng * d2r

	p.year = r.Year
	p.month = r.Month - 1
	p.SSN = r.SSN
	p.txpower = r.TxPower
	p.BW = r.Bandwidth
	p.SNRr = r.RequiredSNR
	p.SNRXXp = r.SNRXXPercent
	p.noiseP.ManMadeNoise = r.ManMadeNoise
	p.Modulation = int(r.Modulation)
	p.SIRr = r.Digital.RequiredSIR
	p.A = r.Digital.A
	p.TW = r.Digital.TW
	p.FW = r.Digital.FW
	p.T0 = r.Digital.T0
	p.F0 = r.Digital.F0
	if r.LongPath {
		p.SorL = longPath
	} else {
		p.SorL = shortPath
	}

	// The main beams point at each other unless the bearing is given.
	txBearing := bearing(p.L_tx, p.L_rx, p.SorL)
	rxBearing := bearing(p.L_rx, p.L_tx, p.SorL)
	if r.TxAntenna.Bearing != nil {
		txBearing = *r.TxAntenna.Bearing * d2r
	}
	if r.RxAntenna.Bearing != nil {
		rxBearing = *r.RxAntenna.Bearing * d2r
	}
	if txBearing > 2.0*pi || txBearing < 0.0 || rxBearing > 2.0*pi || rxBearing < 0.0 || math.IsNaN(txBearing) || math.IsNaN(rxBearing) {
		return nil, ErrBearing
	}

	var err error
	rx, err := antennaPattern(&r.RxAntenna, rxBearing)
	if err != nil {
		return nil, fmt.Errorf("receive antenna: %w", err)
	}
	tx, err := antennaPattern(&r.TxAntenna, txBearing)
	if err != nil {
		return nil, fmt.Errorf("transmit antenna: %w", err)
	}
	p.A_rx = *rx
	p.A_tx = *tx

	return p, nil
}
