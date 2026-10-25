package p372

// tinyDB is the smallest dB value, used as the "not computed" indicator
// (TINYDB in Common.h, DBL_MIN_10_EXP).
const tinyDB = -307.0

// Initialize sets all the output fields to the "not computed" indicator.
func (n *Noise) Initialize() {
	n.FaA = tinyDB
	n.FaG = tinyDB
	n.FaM = tinyDB
	n.FamT = tinyDB

	n.DlA = tinyDB
	n.DlG = tinyDB
	n.DlM = tinyDB
	n.DuG = tinyDB
	n.DuM = tinyDB
	n.DlT = tinyDB
	n.DuA = tinyDB
	n.DuT = tinyDB
}
