package hf

import "fmt"

// ModeResult holds the values of one propagation mode of a point.
type ModeResult struct {
	Name        string  // e.g. "1E" or "2F2": hops and layer
	BMUF        float64 // basic MUF (MHz)
	MUF90       float64 // MUF exceeded for 90% of the days (MHz)
	MUF50       float64 // MUF exceeded for 50% of the days (MHz)
	MUF10       float64 // MUF exceeded for 10% of the days (MHz)
	OPMUF       float64 // operational MUF (MHz)
	OPMUF10     float64
	OPMUF90     float64
	Fprob       float64 // probability that the mode is supported at the frequency (%)
	Height      float64 // reflection height (km)
	ScreeningHz float64 // E layer screening frequency (MHz), F2 modes only
	Loss        float64 // basic transmission loss (dB), paths up to 9000 km
	FieldStr    float64 // field strength Ew (dB(1 uV/m))
	Elevation   float64 // elevation angle (degrees)
	Power       float64 // received power Prw (dBW)
	RxGain      float64 // receive antenna gain (dBi)
	Delay       float64 // time delay (ms)
	Considered  bool    // the mode takes part in the field strength sum
}

// Point is the prediction for one hour and one frequency; the values are
// those of the report of the C driver (the angles are in degrees).
type Point struct {
	Month     int     // 1 to 12
	Hour      int     // UTC hour, 1 to 24
	Frequency float64 // MHz

	Distance    float64 // great circle distance (km)
	Dmax        float64 // dmax (km)
	SlantRange  float64 // slant range (km)
	Elevation   float64 // path elevation angle (degrees)
	BMUF        float64 // basic MUF (MHz)
	MUF50       float64
	MUF90       float64
	MUF10       float64
	OPMUF       float64 // operational MUF (MHz)
	OPMUF90     float64
	OPMUF10     float64
	LowestF2Hop int // hops of the lowest order F2 mode, 0 when there is none
	LowestEHop  int // hops of the lowest order E mode, 0 when there is none

	FieldStrength float64 // path field strength Ep (dB(1 uV/m)): Es, El or Ei by distance
	Es            float64 // short path field strength (dB(1 uV/m))
	El            float64 // long path field strength (dB(1 uV/m))
	Ei            float64 // interpolated field strength, 7000 to 9000 km (dB(1 uV/m))
	Pr            float64 // median available receiver power (dBW)
	RxGain        float64 // receive antenna gain Grw (dBi)

	NoiseAtmospheric float64 // FaA (dB)
	NoiseManMade     float64 // FaM (dB)
	NoiseGalactic    float64 // FaG (dB)
	DuA, DlA         float64 // atmospheric noise deciles (dB)
	DuM, DlM         float64 // man-made noise deciles (dB)
	DuG, DlG         float64 // galactic noise deciles (dB)
	DuT, DlT         float64 // total noise deciles (dB)
	NoiseTotal       float64 // total noise FamT (dB)

	SNR   float64 // median signal-to-noise ratio (dB)
	DuSN  float64 // upper decile deviation (dB)
	DlSN  float64 // lower decile deviation (dB)
	SNRXX float64 // signal-to-noise ratio at the requested time percentage (dB)

	SIR  float64 // signal-to-interference ratio (dB)
	DuSI float64
	DlSI float64
	RSN  float64 // probability that the required SNR is achieved (%)
	RT   float64 // probability that the required time spread is not exceeded (%)
	RF   float64 // probability that the required frequency spread is not exceeded (%)

	BCR      float64 // basic circuit reliability (%)
	OCR      float64 // overall circuit reliability without scattering (%)
	OCRs     float64 // overall circuit reliability with scattering (%)
	MIR      float64 // multimode interference ratio (%)
	ProbOcc  float64 // probability of scattering (%)
	Dominant string  // dominant mode, e.g. "2F2" or "NONE"

	// The values of the dominant mode, zero when there is none.
	DomElevation float64 // degrees
	DomDelay     float64 // ms
	DomLoss      float64 // dB
	DomFprob     float64 // %
	DomHeight    float64 // km
	DomPower     float64 // dBW
	DomRxGain    float64 // dBi
	DomFieldStr  float64 // dB(1 uV/m)
	DomBMUF      float64 // MHz

	// The long path (more than 7000 km) parameters.
	E0  float64    // free space field strength for 3 MW EIRP (dB(1 uV/m))
	Gap float64    // focusing gain (dB)
	Ly  float64    // "not otherwise included" loss (dB)
	FM  float64    // upper reference frequency (MHz)
	FL  float64    // lower reference frequency (MHz)
	K   [2]float64 // correction factors
	Gtl float64    // largest transmit gain between 0 and 8 degrees (dBi)
	FH  float64    // mean gyrofrequency (MHz)
	F   float64

	Modes []ModeResult // the modes that exist (BMUF != 0), E modes first
}

// Result is the output of Predict: one point per hour and frequency, ordered
// by hour (as requested) and then frequency.
type Result struct {
	Points []Point
}

// modeResult converts a mode of the core.
func modeResult(m *mode, name string) ModeResult {
	return ModeResult{
		Name:        name,
		BMUF:        m.BMUF,
		MUF90:       m.MUF90,
		MUF50:       m.MUF50,
		MUF10:       m.MUF10,
		OPMUF:       m.OPMUF,
		OPMUF10:     m.OPMUF10,
		OPMUF90:     m.OPMUF90,
		Fprob:       m.Fprob,
		Height:      m.hr,
		ScreeningHz: m.fs,
		Loss:        m.Lb,
		FieldStr:    m.Ew,
		Elevation:   m.ele * r2d,
		Power:       m.Prw,
		RxGain:      m.Grw,
		Delay:       m.tau,
		Considered:  m.MC != 0,
	}
}

// newPoint collects the report values of a finished calculation.
func newPoint(p *pathData) Point {
	pt := Point{
		Month:     p.month + 1,
		Hour:      p.hour + 1,
		Frequency: p.frequency,

		Distance:    p.distance,
		Dmax:        p.dmax,
		SlantRange:  p.ptick,
		Elevation:   p.ele * r2d,
		BMUF:        p.BMUF,
		MUF50:       p.MUF50,
		MUF90:       p.MUF90,
		MUF10:       p.MUF10,
		OPMUF:       p.OPMUF,
		OPMUF90:     p.OPMUF90,
		OPMUF10:     p.OPMUF10,
		LowestF2Hop: 0,
		LowestEHop:  0,

		FieldStrength: p.Ep,
		Es:            p.Es,
		El:            p.El,
		Ei:            p.Ei,
		Pr:            p.Pr,
		RxGain:        p.Grw,

		NoiseAtmospheric: p.noiseP.FaA,
		NoiseManMade:     p.noiseP.FaM,
		NoiseGalactic:    p.noiseP.FaG,
		DuA:              p.noiseP.DuA,
		DlA:              p.noiseP.DlA,
		DuM:              p.noiseP.DuM,
		DlM:              p.noiseP.DlM,
		DuG:              p.noiseP.DuG,
		DlG:              p.noiseP.DlG,
		DuT:              p.noiseP.DuT,
		DlT:              p.noiseP.DlT,
		NoiseTotal:       p.noiseP.FamT,

		SNR:   p.SNR,
		DuSN:  p.DuSN,
		DlSN:  p.DlSN,
		SNRXX: p.SNRXX,

		SIR:  p.SIR,
		DuSI: p.DuSI,
		DlSI: p.DlSI,
		RSN:  p.RSN,
		RT:   p.RT,
		RF:   p.RF,

		BCR:     p.BCR,
		OCR:     p.OCR,
		OCRs:    p.OCRs,
		MIR:     p.MIR,
		ProbOcc: p.probocc,

		E0:  p.E0,
		Gap: p.Gap,
		Ly:  p.Ly,
		FM:  p.fM,
		FL:  p.fL,
		K:   p.K,
		Gtl: p.Gtl,
		FH:  p.fH,
		F:   p.F,
	}

	if p.n0_F2 != noLowestMode {
		pt.LowestF2Hop = p.n0_F2 + 1
	}
	if p.n0_E != noLowestMode {
		pt.LowestEHop = p.n0_E + 1
	}

	// The dominant mode: 0-2 are E modes, 3-8 F2 modes.
	switch {
	case p.DMidx < maxEMds:
		pt.Dominant = fmt.Sprintf("%dE", p.DMidx+1)
	case p.DMidx <= maxMds:
		pt.Dominant = fmt.Sprintf("%dF2", p.DMidx-2)
	default:
		pt.Dominant = "NONE"
	}
	if p.DMidx <= maxMds && p.DMptr != nil {
		pt.DomElevation = p.DMptr.ele * r2d
		pt.DomDelay = p.DMptr.tau
		pt.DomLoss = p.DMptr.Lb
		pt.DomFprob = p.DMptr.Fprob
		pt.DomHeight = p.DMptr.hr
		pt.DomPower = p.DMptr.Prw
		pt.DomRxGain = p.DMptr.Grw
		pt.DomFieldStr = p.DMptr.Ew
		pt.DomBMUF = p.DMptr.BMUF
	}

	for i := range p.Md_E {
		if p.Md_E[i].BMUF != 0.0 {
			pt.Modes = append(pt.Modes, modeResult(&p.Md_E[i], fmt.Sprintf("%dE", i+1)))
		}
	}
	for i := range p.Md_F2 {
		if p.Md_F2[i].BMUF != 0.0 {
			pt.Modes = append(pt.Modes, modeResult(&p.Md_F2[i], fmt.Sprintf("%dF2", i+1)))
		}
	}

	return pt
}
