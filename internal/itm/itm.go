package itm

import "fmt"

const (
	pi            = 3.1415926535897932384
	a_0__meter    = 6370e3
	a_9000__meter = 9000e3
	third         = 1.0 / 3.0

	mode__P2P  = 0
	mode__AREA = 1
)

type Polarization int

const (
	Horizontal Polarization = 0
	Vertical   Polarization = 1
)

type Climate int

const (
	Equatorial                Climate = 1
	ContinentalSubtropical    Climate = 2
	MaritimeSubtropical       Climate = 3
	Desert                    Climate = 4
	ContinentalTemperate      Climate = 5
	MaritimeTemperateOverLand Climate = 6
	MaritimeTemperateOverSea  Climate = 7
)

const (
	SingleMessageMode = 0
	AccidentalMode    = 1
	MobileMode        = 2
	BroadcastMode     = 3
)

type Mode int

const (
	ModeNotSet       Mode = 0
	ModeLineOfSight  Mode = 1
	ModeDiffraction  Mode = 2
	ModeTroposcatter Mode = 3
)

func (m Mode) String() string {
	switch m {
	case ModeLineOfSight:
		return "line of sight"
	case ModeDiffraction:
		return "diffraction"
	case ModeTroposcatter:
		return "troposcatter"
	}
	return "not set"
}

type Params struct {
	HTxM, HRxM                float64
	FreqMHz                   float64
	Polarization              Polarization
	Epsilon                   float64
	Sigma                     float64
	N0                        float64
	Climate                   Climate
	Mdvar                     int
	Time, Location, Situation float64
}

type Result struct {
	LossDB   float64
	Mode     Mode
	Warnings Warning

	ThetaHzn   [2]float64
	TakeoffRad [2]float64
	DHznM      [2]float64
	HeM        [2]float64
	Ns         float64
	DeltaHM    float64
	ARefDB     float64
	AFsDB      float64
	DistanceKm float64
}

func PointToPoint(elev []float64, stepM float64, p Params) (Result, error) {
	if len(elev) < 2 {
		return Result{}, fmt.Errorf("itm: profile needs at least 2 points, got %d", len(elev))
	}
	if stepM <= 0 {
		return Result{}, fmt.Errorf("itm: step must be positive, got %v", stepM)
	}
	pfl := make([]float64, len(elev)+2)
	pfl[0] = float64(len(elev) - 1)
	pfl[1] = stepM
	copy(pfl[2:], elev)
	return pointToPoint(pfl, p)
}

func pointToPoint(pfl []float64, p Params) (Result, error) {
	var r Result
	var warnings Warning

	rtn := validateInputs(p.HTxM, p.HRxM, int(p.Climate), p.Time, p.Location, p.Situation, p.N0, p.FreqMHz,
		int(p.Polarization), p.Epsilon, p.Sigma, p.Mdvar, &warnings)
	if rtn != success {
		return r, codeError(rtn)
	}

	r.DistanceKm = (pfl[0] * pfl[1]) / 1000

	np := int(pfl[0])

	p10 := int(0.1 * float64(np))
	h_sys__meter := 0.0
	for i := p10; i <= np-p10; i++ {
		h_sys__meter += pfl[i+2]
	}
	h_sys__meter = h_sys__meter / float64(np-2*p10+1)

	Z_g, gamma_e, N_s := initializePointToPoint(p.FreqMHz, h_sys__meter, p.N0, int(p.Polarization), p.Epsilon, p.Sigma)

	h__meter := [2]float64{p.HTxM, p.HRxM}
	var theta_hzn, d_hzn__meter, h_e__meter, takeoff [2]float64
	var delta_h__meter, d__meter float64
	quickPfl(pfl, gamma_e, h__meter, &theta_hzn, &d_hzn__meter, &h_e__meter, &delta_h__meter, &d__meter, &takeoff)

	A_ref__db := 0.0
	propmode := ModeNotSet
	rtn = longleyRice(theta_hzn, p.FreqMHz, Z_g, d_hzn__meter, h_e__meter, gamma_e, N_s, delta_h__meter, h__meter,
		d__meter, mode__P2P, &A_ref__db, &warnings, &propmode)
	if rtn != success {
		return r, codeError(rtn)
	}

	A_fs__db := freeSpaceLoss(d__meter, p.FreqMHz)

	r.LossDB = variability(p.Time, p.Location, p.Situation, h_e__meter, delta_h__meter, p.FreqMHz, d__meter,
		A_ref__db, int(p.Climate), p.Mdvar, &warnings) + A_fs__db

	r.ARefDB = A_ref__db
	r.AFsDB = A_fs__db
	r.DeltaHM = delta_h__meter
	r.DHznM = d_hzn__meter
	r.HeM = h_e__meter
	r.Ns = N_s
	r.ThetaHzn = theta_hzn
	r.TakeoffRad = takeoff
	r.Mode = propmode
	r.Warnings = warnings
	return r, nil
}

func FreeSpaceLoss(distanceM, freqMHz float64) float64 {
	return freeSpaceLoss(distanceM, freqMHz)
}

func fdim(x, y float64) float64 {
	if x > y {
		return x - y
	}
	return 0
}
