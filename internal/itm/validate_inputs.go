package itm

func validateInputs(h_tx__meter, h_rx__meter float64, climate int, time, location, situation, N_0, f__mhz float64,
	pol int, epsilon, sigma float64, mdvar int, warnings *Warning) int {
	if h_tx__meter < 1.0 || h_tx__meter > 1000.0 {
		*warnings |= WarnTxTerminalHeight
	}
	if h_tx__meter < 0.5 || h_tx__meter > 3000.0 {
		return error__TX_TERMINAL_HEIGHT
	}

	if h_rx__meter < 1.0 || h_rx__meter > 1000.0 {
		*warnings |= WarnRxTerminalHeight
	}
	if h_rx__meter < 0.5 || h_rx__meter > 3000.0 {
		return error__RX_TERMINAL_HEIGHT
	}

	if climate < int(Equatorial) || climate > int(MaritimeTemperateOverSea) {
		return error__INVALID_RADIO_CLIMATE
	}

	if N_0 < 250 || N_0 > 400 {
		return error__REFRACTIVITY
	}

	if f__mhz < 40.0 || f__mhz > 10000.0 {
		*warnings |= WarnFrequency
	}
	if f__mhz < 20 || f__mhz > 20000 {
		return error__FREQUENCY
	}

	if pol != int(Horizontal) && pol != int(Vertical) {
		return error__POLARIZATION
	}

	if epsilon < 1 {
		return error__EPSILON
	}

	if sigma <= 0 {
		return error__SIGMA
	}

	if mdvar < 0 ||
		(mdvar > 3 && mdvar < 10) ||
		(mdvar > 13 && mdvar < 20) ||
		(mdvar > 23 && mdvar < 30) ||
		mdvar > 33 {
		return error__MDVAR
	}

	if situation <= 0 || situation >= 100 {
		return error__INVALID_SITUATION
	}
	if time <= 0 || time >= 100 {
		return error__INVALID_TIME
	}
	if location <= 0 || location >= 100 {
		return error__INVALID_LOCATION
	}

	return success
}
