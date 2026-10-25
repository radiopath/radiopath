package hf

// mufOperational calculates the operational MUF of every mode and of the path
// (P.533 section 3.7).
func mufOperational(p *pathData) {
	// Table 1 of ITU-R P.1240-1: ratio of the median operational MUF to the
	// median basic MUF of an F2 mode, Rop[power][season][day/night].
	rop := [2][3][2]float64{
		{
			{1.20, 1.30},
			{1.15, 1.25},
			{1.10, 1.20},
		},
		{
			{1.15, 1.25},
			{1.20, 1.30},
			{1.25, 1.35},
		},
	}

	var opEMUF, opEMUF10, opEMUF90 float64
	var opF2MUF, opF2MUF10, opF2MUF90 float64

	// Only for paths up to 9000 km.
	if p.distance > 9000 {
		return
	}

	// Day when the local time is between sunrise and sunset, else night.
	var tm int
	if (p.CP[mp].ltime < p.CP[mp].Sun.lss && p.CP[mp].ltime > p.CP[mp].Sun.lsr) &&
		!(p.CP[mp].ltime > p.CP[mp].Sun.lss && p.CP[mp].ltime < p.CP[mp].Sun.lsr) {
		tm = day
	} else {
		tm = night
	}

	// The EIRP index; both branches of the C code select the same table.
	power := 0

	// The F2 modes: set the OPMUF of the mode and track the largest.
	opF2MUF = 0.0
	opF2MUF10 = 0.0
	opF2MUF90 = 0.0

	for i := 0; i < maxF2Mds; i++ {
		if p.Md_F2[i].BMUF != 0.0 {
			p.Md_F2[i].OPMUF = p.Md_F2[i].MUF50 * rop[power][p.season][tm]
			p.Md_F2[i].OPMUF10 = p.Md_F2[i].OPMUF * p.Md_F2[i].deltau
			p.Md_F2[i].OPMUF90 = p.Md_F2[i].OPMUF * p.Md_F2[i].deltal

			if p.Md_F2[i].OPMUF > opF2MUF {
				opF2MUF = p.Md_F2[i].OPMUF
			}
			if p.Md_F2[i].OPMUF90 > opF2MUF90 {
				opF2MUF90 = p.Md_F2[i].OPMUF90
			}
			if p.Md_F2[i].OPMUF10 > opF2MUF10 {
				opF2MUF10 = p.Md_F2[i].OPMUF10
			}
		}
	}

	// The E modes
	opEMUF = 0.0
	opEMUF10 = 0.0
	opEMUF90 = 0.0

	for i := 0; i < maxEMds; i++ {
		if p.Md_E[i].BMUF != 0.0 {
			p.Md_E[i].OPMUF = p.Md_E[i].BMUF
			p.Md_E[i].OPMUF10 = p.Md_E[i].OPMUF * p.Md_E[i].deltau
			p.Md_E[i].OPMUF90 = p.Md_E[i].OPMUF * p.Md_E[i].deltal

			if p.Md_E[i].OPMUF > opEMUF {
				opEMUF = p.Md_E[i].OPMUF
			}
			if p.Md_E[i].OPMUF90 > opEMUF90 {
				opEMUF90 = p.Md_E[i].OPMUF90
			}
			if p.Md_E[i].OPMUF10 > opEMUF10 {
				opEMUF10 = p.Md_E[i].OPMUF10
			}
		}
	}

	// The largest OPMUF of all existing modes.
	p.OPMUF = cMax(opEMUF, opF2MUF)
	p.OPMUF90 = cMax(opEMUF90, opF2MUF90)
	p.OPMUF10 = cMax(opEMUF10, opF2MUF10)
}
