package hf

import "github.com/radiopath/radiopath/internal/hf/p372"

// calculate runs the P.533 calculation engine on the path (P533() of the C
// code). The data arrays, the antenna patterns and the user input must be
// loaded in the path before. It performs no I/O.
//
// Part 1 finds the frequency availability, part 2 the median sky-wave field
// strength and part 3 predicts the system performance. The routines populate
// the path structure as the calculation progresses, so they have to run in
// this order.
func calculate(p *pathData) error {
	// Validate the input data.
	if err := validatePath(p); err != nil {
		return err
	}

	// The distance, the mid-point of the path and the ionospheric parameters of
	// three of the potential five control points.
	initializePath(p)

	// Part 1 - Frequency availability. Only for paths up to 9000 km.

	// The basic MUF; also determines R - d0/2 and T + d0/2 when necessary.
	mufBasic(p)

	// Fprob of each mode and the 50%, 90% and 10% MUF of the path.
	mufVariability(p)

	// The operational MUF and its 90% and 10% values.
	mufOperational(p)

	// The E layer screening frequency, depending on the path length.
	eLayerScreeningFrequency(p)

	// Part 2 - Median sky-wave field strength. Each routine checks the path
	// distance to decide whether it applies.
	medianSkywaveFieldStrengthShort(p)

	medianSkywaveFieldStrengthLong(p)

	between7000kmand9000km(p)

	medianAvailableReceiverPower(p)

	// Part 3 - The prediction of system performance.

	// The noise at the receiver.
	p.noiseP.Hour = p.hour
	p.noiseP.Lng = p.L_rx.lng
	p.noiseP.Lat = p.L_rx.lat
	p.noiseP.Frequency = p.frequency
	if err := p372.Compute(&p.noiseP, p.coeff); err != nil {
		return err
	}

	circuitReliability(p)

	return nil
}
