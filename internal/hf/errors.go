package hf

import "errors"

// Errors of ValidatePath(), the data loaders and the antenna readers. The
// comments name the RTN_ERR* code of the C reference.
var (
	ErrYear              = errors.New("hf: invalid year (1900..2100)")                      // RTN_ERRYEAR
	ErrMonth             = errors.New("hf: invalid month")                                  // RTN_ERRMONTH
	ErrHour              = errors.New("hf: invalid hour")                                   // RTN_ERRHOUR
	ErrManMadeNoise      = errors.New("hf: invalid man-made noise")                         // RTN_ERRMANMADENOISE
	ErrNoFoF2Data        = errors.New("hf: missing foF2 array data")                        // RTN_ERRNOFOF2DATA
	ErrNoM3kF2Data       = errors.New("hf: missing M(3000)F2 array data")                   // RTN_ERRNOM3KF2DATA
	ErrNoDudData         = errors.New("hf: missing DuD array data")                         // RTN_ERRNODUDDATA
	ErrNoFamData         = errors.New("hf: missing Fam array data")                         // RTN_ERRNOFAMDATA
	ErrNoFoF2VarData     = errors.New("hf: missing foF2 variability array data")            // RTN_ERRNOFOF2VARDATA
	ErrSSN               = errors.New("hf: invalid sunspot number (1..311)")                // RTN_ERRSSN
	ErrModulation        = errors.New("hf: invalid modulation")                             // RTN_ERRMODULATION
	ErrFrequency         = errors.New("hf: invalid frequency (1..30 MHz)")                  // RTN_ERRFREQUENCY
	ErrBandwidth         = errors.New("hf: invalid bandwidth (0.005 Hz..3 MHz)")            // RTN_ERRBW
	ErrTxPower           = errors.New("hf: invalid transmit power (-30..60 dB(kW))")        // RTN_ERRTXPOWER
	ErrSNRRequired       = errors.New("hf: invalid required signal-to-noise ratio")         // RTN_ERRSNRR
	ErrSIRRequired       = errors.New("hf: invalid required signal-to-interference ratio")  // RTN_ERRSIRR
	ErrF0                = errors.New("hf: invalid frequency dispersion F0")                // RTN_ERRF0
	ErrT0                = errors.New("hf: invalid time spread T0")                         // RTN_ERRT0
	ErrA                 = errors.New("hf: invalid digital modulation amplitude ratio A")   // RTN_ERRA
	ErrTW                = errors.New("hf: invalid digital modulation time window TW")      // RTN_ERRTW
	ErrFW                = errors.New("hf: invalid digital modulation frequency window FW") // RTN_ERRFW
	ErrTxLocation        = errors.New("hf: invalid transmit location")                      // RTN_ERRLTX
	ErrRxLocation        = errors.New("hf: invalid receive location")                       // RTN_ERRLRX
	ErrRxAntennaPattern  = errors.New("hf: invalid receive antenna pattern")                // RTN_ERRRXANTENNAPATTERN
	ErrTxAntennaPattern  = errors.New("hf: invalid transmit antenna pattern")               // RTN_ERRTXANTENNAPATTERN
	ErrSNRXXp            = errors.New("hf: invalid SNRXX percentage (1..99)")               // RTN_ERRSNRXXP
	ErrBearing           = errors.New("hf: antenna bearing out of range (0..360 degrees)")  // RTN_ERRTXBEARING, RTN_ERRRXBEARING
	ErrGainOffset        = errors.New("hf: antenna gain offset out of range")               // RTN_ERRTXGOS, RTN_ERRRXGOS
	ErrOpenAntennaFile   = errors.New("hf: cannot open antenna file")                       // RTN_ERRCANTOPENANTFILE
	ErrAntennaType       = errors.New("hf: unsupported antenna file type")                  // Unsupported antenna type
	ErrOpenP1239File     = errors.New("hf: cannot open 'P1239-3 Decile Factors.txt'")       // RTN_ERRCANTOPENP1239FILE
	ErrNotP12393         = errors.New("hf: invalid P.1239-3 file")                          // RTN_ERRNOTP12393
	ErrReadIonParameters = errors.New("hf: cannot read ionospheric parameters file")        // RTN_ERRREADIONPARAMETERS
	ErrNoHours           = errors.New("hf: no valid hour in the request")
	ErrNoFrequencies     = errors.New("hf: no valid frequency in the request")
)
