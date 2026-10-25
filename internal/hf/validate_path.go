package hf

import "math"

// validatePath checks the data in the path for correct input and returns the
// error that ValidatePath() of the C code reports as a return code.
func validatePath(p *pathData) error {
	if p.year < 1900 || p.year > 2100 {
		return ErrYear
	}
	if p.month < 0 || p.month > 11 {
		return ErrMonth
	}
	if p.hour < 0 || p.hour > 23 {
		return ErrHour
	}

	// The C condition for an invalid man-made noise can never be true (its
	// terms exclude each other), so there is no error to return here.

	if p.foF2 == nil {
		return ErrNoFoF2Data
	}
	if p.M3kF2 == nil {
		return ErrNoM3kF2Data
	}
	if p.coeff == nil {
		return ErrNoDudData
	}
	if p.foF2var == nil {
		return ErrNoFoF2VarData
	}
	if 1 > p.SSN || p.SSN > 311 {
		return ErrSSN
	}
	if p.Modulation != digital && p.Modulation != analog {
		return ErrModulation
	}
	if 1.0 > p.frequency || p.frequency > 30.0 {
		return ErrFrequency
	}
	if 0.005 > p.BW || p.BW > 3e6 {
		return ErrBandwidth
	}
	if -30.0 > p.txpower || p.txpower > 60 {
		return ErrTxPower
	}
	if -30.0 > p.SNRr || p.SNRr > 200 {
		return ErrSNRRequired
	}
	if -30.0 > p.SIRr || p.SIRr > 200 {
		return ErrSIRRequired
	}
	if 0.0 > p.F0 || p.F0 > 1000 {
		return ErrF0
	}
	if 0.0 > p.T0 || p.T0 > 1000 {
		return ErrT0
	}
	if 0.0 > p.A || p.A > 1000 {
		return ErrA
	}
	if 0.0 > p.TW || p.TW > 50.0 {
		return ErrTW
	}
	if 0.0 > p.FW || p.FW > 1000 {
		return ErrFW
	}
	if math.Abs(p.L_tx.lat) > pi/2.0 || math.Abs(p.L_tx.lng) > pi {
		return ErrTxLocation
	}
	if math.Abs(p.L_rx.lat) > pi/2.0 || math.Abs(p.L_rx.lng) > pi {
		return ErrRxLocation
	}
	if p.A_rx.pattern == nil {
		return ErrRxAntennaPattern
	}
	if p.A_tx.pattern == nil {
		return ErrTxAntennaPattern
	}
	if 1 > p.SNRXXp || p.SNRXXp > 99 {
		return ErrSNRXXp
	}

	return nil
}
