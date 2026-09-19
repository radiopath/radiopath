package itm

import "errors"

const (
	success = 0

	error__TX_TERMINAL_HEIGHT         = 1000
	error__RX_TERMINAL_HEIGHT         = 1001
	error__INVALID_RADIO_CLIMATE      = 1002
	error__INVALID_TIME               = 1003
	error__INVALID_LOCATION           = 1004
	error__INVALID_SITUATION          = 1005
	error__REFRACTIVITY               = 1008
	error__FREQUENCY                  = 1009
	error__POLARIZATION               = 1010
	error__EPSILON                    = 1011
	error__SIGMA                      = 1012
	error__GROUND_IMPEDANCE           = 1013
	error__MDVAR                      = 1014
	error__EFFECTIVE_EARTH            = 1016
	error__SURFACE_REFRACTIVITY_SMALL = 1021
	error__SURFACE_REFRACTIVITY_LARGE = 1022
)

var (
	ErrTxTerminalHeight         = errors.New("itm: TX terminal height is out of range (0.5..3000 m)")
	ErrRxTerminalHeight         = errors.New("itm: RX terminal height is out of range (0.5..3000 m)")
	ErrInvalidRadioClimate      = errors.New("itm: invalid radio climate")
	ErrInvalidTime              = errors.New("itm: time percentage is out of range")
	ErrInvalidLocation          = errors.New("itm: location percentage is out of range")
	ErrInvalidSituation         = errors.New("itm: situation percentage is out of range")
	ErrRefractivity             = errors.New("itm: refractivity is out of range (250..400)")
	ErrFrequency                = errors.New("itm: frequency is out of range (20..20000 MHz)")
	ErrPolarization             = errors.New("itm: invalid polarization")
	ErrEpsilon                  = errors.New("itm: epsilon is out of range")
	ErrSigma                    = errors.New("itm: sigma is out of range")
	ErrGroundImpedance          = errors.New("itm: imaginary part of ground impedance larger than real part")
	ErrMdvar                    = errors.New("itm: invalid mode of variability")
	ErrEffectiveEarth           = errors.New("itm: computed effective earth radius is invalid")
	ErrSurfaceRefractivitySmall = errors.New("itm: computed surface refractivity is too small")
	ErrSurfaceRefractivityLarge = errors.New("itm: computed surface refractivity is too large")
)

var codeErrors = map[int]error{
	error__TX_TERMINAL_HEIGHT:         ErrTxTerminalHeight,
	error__RX_TERMINAL_HEIGHT:         ErrRxTerminalHeight,
	error__INVALID_RADIO_CLIMATE:      ErrInvalidRadioClimate,
	error__INVALID_TIME:               ErrInvalidTime,
	error__INVALID_LOCATION:           ErrInvalidLocation,
	error__INVALID_SITUATION:          ErrInvalidSituation,
	error__REFRACTIVITY:               ErrRefractivity,
	error__FREQUENCY:                  ErrFrequency,
	error__POLARIZATION:               ErrPolarization,
	error__EPSILON:                    ErrEpsilon,
	error__SIGMA:                      ErrSigma,
	error__GROUND_IMPEDANCE:           ErrGroundImpedance,
	error__MDVAR:                      ErrMdvar,
	error__EFFECTIVE_EARTH:            ErrEffectiveEarth,
	error__SURFACE_REFRACTIVITY_SMALL: ErrSurfaceRefractivitySmall,
	error__SURFACE_REFRACTIVITY_LARGE: ErrSurfaceRefractivityLarge,
}

func codeError(code int) error {
	if err, ok := codeErrors[code]; ok {
		return err
	}
	return errors.New("itm: unknown error code")
}

type Warning int64

const (
	WarnTxTerminalHeight      Warning = 0x0001
	WarnRxTerminalHeight      Warning = 0x0002
	WarnFrequency             Warning = 0x0004
	WarnPathDistanceTooBig1   Warning = 0x0008
	WarnPathDistanceTooBig2   Warning = 0x0010
	WarnPathDistanceTooSmall1 Warning = 0x0020
	WarnPathDistanceTooSmall2 Warning = 0x0040
	WarnTxHorizonAngle        Warning = 0x0080
	WarnRxHorizonAngle        Warning = 0x0100
	WarnTxHorizonDistance1    Warning = 0x0200
	WarnRxHorizonDistance1    Warning = 0x0400
	WarnTxHorizonDistance2    Warning = 0x0800
	WarnRxHorizonDistance2    Warning = 0x1000
	WarnExtremeVariabilities  Warning = 0x2000
	WarnSurfaceRefractivity   Warning = 0x4000
)

var warningText = []struct {
	flag Warning
	text string
}{
	{WarnTxTerminalHeight, "TX terminal height is near its limits"},
	{WarnRxTerminalHeight, "RX terminal height is near its limits"},
	{WarnFrequency, "frequency is near its limits"},
	{WarnPathDistanceTooBig1, "path distance is near its upper limit"},
	{WarnPathDistanceTooBig2, "path distance is large - care must be taken with result"},
	{WarnPathDistanceTooSmall1, "path distance is near its lower limit"},
	{WarnPathDistanceTooSmall2, "path distance is small - care must be taken with result"},
	{WarnTxHorizonAngle, "TX horizon angle is large - small angle approximations could break down"},
	{WarnRxHorizonAngle, "RX horizon angle is large - small angle approximations could break down"},
	{WarnTxHorizonDistance1, "TX horizon distance is less than 1/10 of the smooth earth horizon distance"},
	{WarnRxHorizonDistance1, "RX horizon distance is less than 1/10 of the smooth earth horizon distance"},
	{WarnTxHorizonDistance2, "TX horizon distance is greater than 3 times the smooth earth horizon distance"},
	{WarnRxHorizonDistance2, "RX horizon distance is greater than 3 times the smooth earth horizon distance"},
	{WarnExtremeVariabilities, "one of the provided variabilities is located far in the tail of its distribution"},
	{WarnSurfaceRefractivity, "computed surface refractivity is small - care must be taken with result"},
}

func (w Warning) Messages() []string {
	var out []string
	for _, wt := range warningText {
		if w&wt.flag != 0 {
			out = append(out, wt.text)
		}
	}
	return out
}
