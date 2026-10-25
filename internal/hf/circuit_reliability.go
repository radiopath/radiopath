package hf

import "math"

const (
	dominant = 0
	soonest  = 1
	notIndex = 99

	dblMin = 2.2250738585072014e-308 // DBL_MIN of Common.h
)

// circuitReliability finds the basic circuit reliability BCR and the overall
// circuit reliability OCR for analog and digital systems.
//
// The BCR is calculated as in Table 1 of ITU-R P.842-4 in 11 steps. Step 1
// (the median available receiver power) is calculated in
// medianAvailableReceiverPower(), steps 2, 5 and 8 (the noise factors and
// their deciles) by the P.372 noise model. This routine completes the other
// steps. For digital modulation it also determines the approximate BCR of
// section 9 of P.842-4, the OCR of Table 3 and the OCR in the presence of
// equatorial scattering.
func circuitReliability(p *pathData) {
	var x, y, s float64
	var fBMUFR float64
	var duSd, dlSd, duSh, dlSh float64

	var dtu, dtl, dfu, dfl float64
	var dlIh, duIh float64
	var tm, fm float64
	var d float64
	var isum, isuml, isumu float64

	// NORM holds the normal cdf independent variables from 0.5 to 0.99 in 0.01
	// increments for a standard deviation of one and a mean of zero.
	norm := normTable

	var iI, iS [maxMds]int // interference and signal mode indices

	// Table 2 of P.842-4
	table2LD := [2][10]float64{
		{8.0, 12.0, 13.0, 10.0, 8.0, 8.0, 8.0, 7.0, 6.0, 5.0},
		{11.0, 16.0, 17.0, 13.0, 11.0, 11.0, 11.0, 9.0, 8.0, 7.0},
	}
	table2UD := [2][10]float64{
		{6.0, 8.0, 12.0, 13.0, 12.0, 9.0, 9.0, 8.0, 7.0, 7.0},
		{9.0, 11.0, 12.0, 13.0, 12.0, 9.0, 9.0, 8.0, 7.0, 7.0},
	}

	var gt60deg int // index for more than 60 degrees geomagnetic latitude
	var bmuf int    // basic MUF index of Table 2

	noiseP := p.noiseP

	// BCR calculation

	// Step 1: median available receiver power of the wanted signal, see
	// medianAvailableReceiverPower().

	// Step 2: median noise factor, see the P.372 noise model.

	// Step 3: median resultant signal-to-noise ratio for bandwidth b (Hz). The
	// signal differs between analog and digital modulation; the BCR
	// calculation is the same afterwards.
	if p.Modulation == analog {
		s = p.Pr
	} else {
		// The signal of digital modulation needs the calculation of P.533
		// section 10.2.3 "Reliability prediction procedure".
		s = digitalModulationSignalandInterferers(p, &iS, &iI)
	}

	// The SNR
	p.SNR = s - 10.0*math.Log10(math.Pow(10.0, (noiseP.FaA/10.0))+math.Pow(10.0, (noiseP.FaM/10.0))+math.Pow(10.0, (noiseP.FaG/10.0))) -
		10.0*math.Log10(p.BW) + 204

	// Steps 4 and 7: signal upper and lower decile deviation (day-to-day).

	// Does the path cross 60 degrees geomagnetic latitude? From note 1 of
	// Table 2 of P.842-4: any point between the control points 1000 km from
	// each end counts.
	gt60deg = 0 // 0 means less than 60 degrees
	if p.distance > 2000.0 {
		// There may be a degenerate case that three locations do not
		// determine correctly.
		geomag := geomagneticCoords(p.CP[mp].L)
		if geomag.lat >= 60.0*d2r {
			gt60deg = 1
		}
		geomag = geomagneticCoords(p.CP[t1k].L)
		if geomag.lat >= 60.0*d2r {
			gt60deg = 1
		}
		geomag = geomagneticCoords(p.CP[r1k].L)
		if geomag.lat >= 60.0*d2r {
			gt60deg = 1
		}
	}

	// The basic MUF index of Table 2 of P.842-4.
	fBMUFR = p.frequency / p.BMUF

	bmuf = 9

	if 0.8 >= fBMUFR {
		bmuf = 0
	} else if 0.8 < fBMUFR && fBMUFR <= 1.0 {
		bmuf = 1
	} else if 1.0 < fBMUFR && fBMUFR <= 1.2 {
		bmuf = 2
	} else if 1.2 < fBMUFR && fBMUFR <= 1.4 {
		bmuf = 3
	} else if 1.4 < fBMUFR && fBMUFR <= 1.6 {
		bmuf = 4
	} else if 1.6 < fBMUFR && fBMUFR <= 1.8 {
		bmuf = 5
	} else if 1.8 < fBMUFR && fBMUFR <= 2.0 {
		bmuf = 6
	} else if 2.0 < fBMUFR && fBMUFR <= 3.0 {
		bmuf = 7
	} else if 3.0 < fBMUFR && fBMUFR <= 4.0 {
		bmuf = 8
	} else if 4.0 < fBMUFR {
		bmuf = 9
	}

	// The deciles day-to-day
	dlSd = table2LD[gt60deg][bmuf]
	duSd = table2UD[gt60deg][bmuf]

	// The deciles hour-to-hour
	duSh = 5.0
	dlSh = 8.0

	// Step 6: upper decile deviation of the resultant signal-to-noise ratio.
	x = math.Pow(10.0, (noiseP.FaA/10.0)) + math.Pow(10.0, (noiseP.FaM/10.0)) + math.Pow(10.0, (noiseP.FaG/10.0))
	y = math.Pow(10.0, ((noiseP.FaA-noiseP.DlA)/10.0)) + math.Pow(10.0, ((noiseP.FaM-noiseP.DlM)/10.0)) + math.Pow(10.0, ((noiseP.FaG-noiseP.DlG)/10.0))

	p.DuSN = math.Sqrt(math.Pow(10.0*math.Log10(x/y), 2) + math.Pow(duSd, 2) + math.Pow(duSh, 2))

	// Step 9: lower decile deviation of the resultant signal-to-noise ratio.
	// The value of x can be reused from step 6.
	y = math.Pow(10.0, ((noiseP.FaA+noiseP.DuA)/10.0)) + math.Pow(10.0, ((noiseP.FaM+noiseP.DuM)/10.0)) + math.Pow(10.0, ((noiseP.FaG+noiseP.DuG)/10.0))

	p.DlSN = math.Sqrt(math.Pow(10.0*math.Log10(y/x), 2) + math.Pow(dlSd, 2) + math.Pow(dlSh, 2))

	// Step 11: basic circuit reliability for S/N >= or < S/Nr (%).
	if p.SNR >= p.SNRr {
		p.BCR = cMin(130.0-80.0/(1.0+((p.SNR-p.SNRr)/p.DlSN)), 100.0)
	} else {
		p.BCR = cMax(80.0/(1.0+((p.SNRr-p.SNR)/p.DuSN))-30.0, 0.0)
	}

	// The simplified approximate BCR of digital modulation systems, from
	// section 9 "BCR for digital modulation systems" of P.842-4.
	if p.Modulation == digital && (p.T0 != 0.0 && p.F0 != 0.0) {
		// SNR0 = p.SNRr, SNRm = p.SNR, Dl = p.DlSN, Du = p.DuSN

		// The time spread
		d = p.distance
		if d <= 2000.0 {
			tm = cMin((2.5e7 * (1.0 - math.Pow((p.frequency/p.BMUF), 2)) * math.Pow(d, -2)), (7.0 - 0.00175*d))
		} else {
			tm = cMin((4.27e-2 * (1.0 - math.Pow((p.frequency/p.BMUF), 2)) * math.Pow(d, 0.65)), 3.5)
		}

		// The frequency spread
		fm = 0.02 * p.frequency * tm

		// The time deciles
		dtu = 0.15 * tm
		dtl = 0.15 * tm

		// The frequency deciles
		dfu = 0.1 * fm
		dfl = 0.1 * fm

		// The probability that the required signal-to-noise ratio is achieved.
		if p.SNR >= p.SNRr {
			p.RSN = cMin(130.0-80.0/(1.0+((p.SNR-p.SNRr)/p.DlSN)), 100.0)
		} else {
			p.RSN = cMax((80.0/(1.0+((p.SNRr-p.SNR)/p.DuSN)) - 30.0), 0.0)
		}

		// The probability that the required time spread T0 at a level of -10 dB
		// relative to the peak signal amplitude is not exceeded.
		if tm >= p.T0 {
			p.RT = cMin((130.0 - 80.0/(1.0+(p.T0-tm)/dtl)), 100.0)
		} else {
			p.RT = cMax((80.0/(1.0+(tm-p.T0)/dtu) - 30.0), 0.0)
		}

		// The probability that the required frequency spread f0 at a level of
		// -10 dB relative to the peak signal amplitude is not exceeded.
		if fm >= p.F0 {
			p.RF = cMin((130.0 - 80.0/(1.0+(p.F0-fm)/dfl)), 100.0)
		} else {
			p.RF = cMax((80.0/(1.0+(fm-p.F0)/dfu) - 30.0), 0.0)
		}
	}

	// OCR calculation
	if p.Modulation == digital && p.distance <= 9000.0 {
		// Table 3 of P.842-4.
		//
		// Three sums are needed, accomplished in one loop: the interference
		// sum with the protection ratio and the same with the upper and with
		// the lower decile deviation.

		// Step 5: all day-to-day deciles are 0 dB. Step 6: the upper decile
		// deviation of the wanted signal, DuSh, is 5 dB above the lower decile
		// deviation of the interfering signal, set to 8 dB.
		dlIh = 8.0

		// Step 9: the same for the lower decile deviation.
		duIh = 5.0

		// The sums of step 4 (median resultant signal-to-interference ratio),
		// step 7 (upper decile deviation) and step 10 (lower decile
		// deviation).
		isum = 0.0
		isumu = 0.0
		isuml = 0.0
		// iI[] groups all E and F2 modes; they are told apart here. In the F2
		// loop the index is offset by 3 for the 3 E modes.
		for n := 0; n < maxMds; n++ {
			if iI[n] != notIndex {
				if iI[n] < maxEMds { // E mode interference
					isum += math.Pow(10.0, ((p.Md_E[iI[n]].Prw - p.A) / 10.0))
					isumu += math.Pow(10.0, ((p.Md_E[iI[n]].Prw - p.A + duIh) / 10.0))
					isuml += math.Pow(10.0, ((p.Md_E[iI[n]].Prw - p.A - dlIh) / 10.0))
				} else { // F2 mode interference
					isum += math.Pow(10.0, ((p.Md_F2[iI[n]-3].Prw - p.A) / 10.0))
					isumu += math.Pow(10.0, ((p.Md_F2[iI[n]-3].Prw - p.A + duIh) / 10.0))
					isuml += math.Pow(10.0, ((p.Md_F2[iI[n]-3].Prw - p.A - dlIh) / 10.0))
				}
			}
		}

		// With all modes as signal there is no interference: return the
		// values set so far.
		if isum == 0.0 {
			p.DuSI = duSh
			p.DlSI = dlSh
			p.MIR = 0.0
			p.OCR = p.BCR * p.MIR / 100.0
			return
		}

		// Step 4: the signal-to-interference ratio.
		p.SIR = s - 10.0*math.Log10(isum)

		// Step 7: the upper decile deviation of the signal-to-interference ratio.
		p.DuSI = math.Sqrt(math.Pow(duSh, 2) + math.Pow(10.0*math.Log10(isum/isuml), 2))

		// Step 10: the lower decile deviation.
		p.DlSI = math.Sqrt(math.Pow(dlSh, 2) + math.Pow(10.0*math.Log10(isumu/isum), 2))

		// Step 12: circuit reliability in the presence of interference only for
		// S/I >= or < S/Ir.
		if p.SIR >= p.SIRr {
			p.MIR = cMin(130.0-80.0/(1.0+((p.SIR-p.SIRr)/p.DlSI)), 100.0)
		} else {
			p.MIR = cMax((80.0/(1.0+((p.SIRr-p.SIR)/p.DuSI)))-30.0, 0.0)
		}

		// The overall circuit reliability in the absence of scattering.
		p.OCR = p.BCR * p.MIR / 100.0

		// The overall circuit reliability with scattering.
		equatorialScattering(p, &iS)
	}

	// The SNR for the required reliability, see "CCIR Report 322 Noise
	// Variation Parameters", Technical Document 2813, D. C. Lawrence, 1995.
	// Note: NORM[40] = 1.28, SNRXX = SNR50 +- t(XX%)*(D_u,l/1.28)
	if p.SNRXXp < 50 {
		p.SNRXX = p.SNR + p.DuSN*norm[50-p.SNRXXp]/norm[40]
	} else {
		p.SNRXX = p.SNR - p.DlSN*norm[p.SNRXXp-50]/norm[40]
	}
}

// modeSort returns in order the indices of the modes that exist, sorted by
// the criterion: dominant (largest field strength first) or soonest (shortest
// delay first). order must be initialised with notIndex by the caller.
func modeSort(m *[maxMds]*mode, order *[maxMds]int, criteria int) {
	var j int

	// The modes that exist, loaded from index 0.
	cnt := 0
	for n := 0; n < maxMds; n++ {
		if m[n].BMUF != 0.0 {
			order[cnt] = n
			cnt++
		}
	}

	if criteria == dominant {
		for n := 0; n < maxMds; n++ {
			if order[n] != notIndex { // the mode exists
				for k := 0; k < maxMds; k++ {
					if order[k] != notIndex {
						if m[order[n]].Ew > m[order[k]].Ew {
							j = order[n]
							order[n] = order[k]
							order[k] = j
						}
					}
				}
			}
		}
	} else if criteria == soonest {
		for n := 0; n < maxMds; n++ {
			if order[n] != notIndex {
				for k := 0; k < maxMds; k++ {
					if order[k] != notIndex {
						if m[order[n]].BMUF != 0.0 && m[order[n]].tau < m[order[k]].tau {
							j = order[n]
							order[n] = order[k]
							order[k] = j
						}
					}
				}
			}
		}
	}
}

// numberofModes counts the modes that exist.
func numberofModes(p *pathData) int {
	count := 0
	for n := 0; n < maxEMds; n++ {
		if p.Md_E[n].BMUF != 0.0 {
			count++
		}
	}
	for n := 0; n < maxF2Mds; n++ {
		if p.Md_F2[n].BMUF != 0.0 {
			count++
		}
	}

	return count
}

// digitalModulationSignalandInterferers returns the signal that satisfies the
// amplitude ratio A and the time window TW. It also returns the indices of
// the modes that meet the signal criteria in iS and of the interfering modes
// in iI, which is used for the overall circuit reliability.
func digitalModulationSignalandInterferers(p *pathData, iS, iI *[maxMds]int) float64 {
	var etw, ssum, s float64
	var dh, hr, delta, psi, ptick float64
	var deltat, deltaA float64

	var m [maxMds]*mode // all modes, independent of the layer

	// The index arrays of the dominant (Ew) and the soonest (tau) mode order.
	var iEw, itau [9]int

	for n := 0; n < maxMds; n++ {
		iEw[n] = notIndex
		itau[n] = notIndex
		iI[n] = notIndex
		iS[n] = notIndex
	}

	// The slant range ptick of medianSkywaveFieldStrengthShort() was
	// calculated under the E layer screening condition, which is not relevant
	// here, so it is calculated again.
	if p.distance <= 9000.0 {
		// The time delay of all modes that exist. First the E modes with the
		// E layer reflection height.
		hr = 110.0
		for n := p.n0_E; n < maxEMds; n++ {
			if p.Md_E[n].BMUF != 0.0 {
				fn := float64(n)
				dh = p.distance / float64(n+1)
				delta = elevationAngle(dh, hr)
				psi = dh / (2.0 * r0)
				ptick = 2.0 * r0 * (math.Sin(psi) / math.Cos(delta-psi))
				p.Md_E[n].tau = (fn + 1) * (ptick / vOfL) * 1000.0
			}
		}

		// The F2 modes; the reflection height of each was calculated in
		// eLayerScreeningFrequency().
		for n := p.n0_F2; n < maxF2Mds; n++ {
			if p.Md_F2[n].BMUF != 0.0 {
				fn := float64(n)
				hr = p.Md_F2[n].hr
				dh = p.distance / float64(n+1)
				delta = elevationAngle(dh, hr)
				psi = dh / (2.0 * r0)
				ptick = 2.0 * r0 * (math.Sin(psi) / math.Cos(delta-psi))
				p.Md_F2[n].tau = (fn + 1) * (ptick / vOfL) * 1000.0
			}
		}

		// With two or more modes.
		if numberofModes(p) >= 2 {
			// The layers do not matter here: point m at all the modes so a
			// single loop can be used.
			for n := 0; n < maxEMds; n++ {
				m[n] = &p.Md_E[n]
			}
			for n := 0; n < maxF2Mds; n++ {
				m[n+3] = &p.Md_F2[n]
			}

			// P.533 section 10.2.3 "Reliability prediction procedure".
			// Step 1: the dominant mode, Ew.
			modeSort(&m, &iEw, dominant) // iEw[0] is the dominant mode

			// Order the modes by time also.
			modeSort(&m, &itau, soonest) // itau[0] is the earliest mode

			// Step 2: all other active modes with strengths above Ew - A (dB)
			// are identified. Step 3: the first arriving mode and all modes
			// within the time window TW from it are identified. Step 4: for
			// paths up to 7000 km the modes arriving within the window are
			// summed in power, between 7000 and 9000 km the interpolation
			// procedure is used. Step 2 is the amplitude criterion and step 3
			// the delay criterion; step 4 says what to do with the modes that
			// meet both.

			// The tau of the modes is in ms, as is TW.
			deltat = (m[itau[0]].tau + p.TW/1000.0)
			// The dominant mode.
			deltaA = (m[iEw[0]].Prw - p.A)

			ssum = 0.0
			for n := 0; n < maxMds; n++ {
				// A mode is a signal when it exists, is within the A ratio of
				// the dominant mode power and arrives within TW of the earliest
				// arriving mode.
				if m[n].BMUF != 0.0 {
					if m[n].Prw >= deltaA && m[n].tau <= deltat {
						ssum += math.Pow(math.Pow(10.0, m[n].Ew/10.0), 2)
						iS[n] = n
					} else {
						// Otherwise it is interference; the index is kept for
						// the signal-to-interference ratio (Table 3 of P.842-4).
						iI[n] = n
					}
				}
			}

			// Etw only if a mode satisfies the criteria, else something small.
			if ssum > 0 {
				etw = 10.0 * math.Log10(math.Sqrt(ssum))
			} else {
				etw = tinyDB
			}

			// The combined mode power Ssum and the antenna gain Grw.
			s = etw + p.Grw - 20.0*math.Log10(p.frequency) - 107.2
		} else {
			// There is only one mode.
			s = p.Pr
		}
	} else {
		s = p.Pr
	}

	// The iI[] and iS[] arrays need to be ordered.
	for n := 0; n < maxMds; n++ {
		for k := 0; k < maxMds; k++ {
			if iI[n] < iI[k] {
				iI[n], iI[k] = iI[k], iI[n]
			}
			if iS[n] < iS[k] {
				iS[n], iS[k] = iS[k], iS[n]
			}
		}
	}

	return s
}

// modeAt returns the mode that the C code reaches with the expression
// Md_F2[i]. The E modes follow the F2 modes in the PathData structure, so an
// index of 6 or more selects an E mode. The C scattering code relies on this
// when it uses the combined mode index (0-8) for the F2 array.
func modeAt(p *pathData, i int) *mode {
	if i < maxF2Mds {
		return &p.Md_F2[i]
	}
	return &p.Md_E[i-maxF2Mds]
}

// equatorialScattering determines the equatorial scattering by the method of
// P.533 section 10.3 "Equatorial Scattering" and the overall circuit
// reliability with scattering. iS holds the indices of the signal modes.
func equatorialScattering(p *pathData, iS *[maxMds]int) {
	var pm float64
	var ptSpread [maxMds]float64 // time spread of the modes within the amplitude and time criteria
	tSpread := 1.0               // standard deviation of the time spread, 1 ms
	var tau, taum float64
	var pfSpread [2]float64 // frequency spread of the dominant mode
	fSpread := 3.0          // standard deviation of the frequency spread, 3 Hz
	var f, fm float64
	var probocc [maxMds]float64
	var biggest, fr, fs, flambdad, ftl float64

	var useCP bool

	// Step 7: the time spread of the modes within the amplitude ratio A and
	// the time window TW. Their indices were found in
	// digitalModulationSignalandInterferers().
	if p.distance <= 9000.0 && iS[0] != notIndex {
		tau = p.TW

		for n := 0; n < maxMds; n++ {
			// PTspread starts at something tiny, DBL_MIN.
			ptSpread[n] = dblMin
			if iS[n] != notIndex {
				if iS[n] < maxEMds {
					// An E mode
					pm = p.Md_E[iS[n]].Ew
					taum = p.Md_E[iS[n]].tau
				} else {
					// An F2 mode
					pm = modeAt(p, iS[n]).Ew
					taum = modeAt(p, iS[n]).tau
				}

				ptSpread[n] = 0.056 * pm * math.Exp(-(math.Pow((tau-taum), 2))/(2.0*math.Pow(tSpread, 2)))
			}
		}

		// Step 8: the frequency spreading of the dominant mode.
		if iS[0] < maxEMds {
			pm = p.Md_E[iS[0]].Ew
		} else {
			pm = modeAt(p, iS[0]).Ew
		}

		// The center frequency (Hz)
		fm = p.frequency * 1e6

		// The frequency window (Hz)
		f = p.FW

		pfSpread[0] = 0.056 * pm * math.Exp(-(math.Pow((+f-fm), 2))/(2.0*math.Pow(fSpread, 2)))
		pfSpread[1] = 0.056 * pm * math.Exp(-(math.Pow((-f-fm), 2))/(2.0*math.Pow(fSpread, 2)))

		// Step 9: the probability of scattering occurring.
		useCP = false
		// Is the time spread component within the amplitude ratio A of the
		// dominant mode power level? Note that pm is the dominant mode here.
		for n := 0; n < maxMds; n++ {
			if ptSpread[n] != dblMin {
				if (pm - ptSpread[n]) >= p.A {
					useCP = true
				}
			}
		}
		// The same for the frequency spread components.
		for n := 0; n < 2; n++ {
			if (pm - pfSpread[n]) >= p.A {
				useCP = true
			}
		}

		// The coefficients of the probocc calculation that do not depend on
		// the control point. The C code uses max() here, not min().
		fr = (0.1 + 0.008*float64(cMaxInt(p.SSN, 160)))
		fs = 0.55 + 0.45*math.Sin(60.0*d2r*((float64(p.month)+1.0)-1.5))

		if useCP { // use the control points
			for n := 0; n < maxMds; n++ { // all modes
				probocc[n] = 0.0
				flambdad = 0.0
				ftl = 0.0

				if iS[n] != notIndex && iS[n] >= maxEMds { // does the mode exist and is it an F2 mode?
					if iS[n] == p.n0_F2 { // the lowest order F2 mode
						if p.distance <= p.dmax {
							flambdad = findFlambdad(p.CP[mp])
							ftl = findFTl(p.CP[mp])
						} else {
							if ptSpread[td02] >= ptSpread[rd02] {
								flambdad = findFlambdad(p.CP[td02])
								ftl = findFTl(p.CP[td02])
							} else {
								flambdad = findFlambdad(p.CP[rd02])
								ftl = findFTl(p.CP[rd02])
							}
						}
					} else { // higher order F2 modes
						if p.distance <= p.dmax {
							// The largest time scattering by brute force.
							if ptSpread[t1k] >= ptSpread[r1k] && ptSpread[t1k] >= ptSpread[mp] {
								flambdad = findFlambdad(p.CP[t1k])
								ftl = findFTl(p.CP[t1k])
							} else if ptSpread[r1k] >= ptSpread[t1k] && ptSpread[r1k] >= ptSpread[mp] {
								flambdad = findFlambdad(p.CP[r1k])
								ftl = findFTl(p.CP[r1k])
							} else if ptSpread[mp] >= ptSpread[r1k] && ptSpread[mp] >= ptSpread[t1k] {
								flambdad = findFlambdad(p.CP[mp])
								ftl = findFTl(p.CP[mp])
							}
						} else {
							if ptSpread[t1k] >= ptSpread[r1k] &&
								ptSpread[t1k] >= ptSpread[td02] &&
								ptSpread[t1k] >= ptSpread[mp] &&
								ptSpread[t1k] >= ptSpread[rd02] {
								flambdad = findFlambdad(p.CP[t1k])
								ftl = findFTl(p.CP[t1k])
							} else if ptSpread[r1k] >= ptSpread[t1k] &&
								ptSpread[r1k] >= ptSpread[mp] &&
								ptSpread[r1k] >= ptSpread[td02] &&
								ptSpread[r1k] >= ptSpread[rd02] {
								flambdad = findFlambdad(p.CP[r1k])
								ftl = findFTl(p.CP[r1k])
							} else if ptSpread[mp] >= ptSpread[r1k] &&
								ptSpread[mp] >= ptSpread[t1k] &&
								ptSpread[mp] >= ptSpread[td02] &&
								ptSpread[mp] >= ptSpread[rd02] {
								flambdad = findFlambdad(p.CP[mp])
								ftl = findFTl(p.CP[mp])
							} else if ptSpread[td02] >= ptSpread[rd02] &&
								ptSpread[td02] >= ptSpread[mp] &&
								ptSpread[td02] >= ptSpread[t1k] &&
								ptSpread[td02] >= ptSpread[r1k] {
								flambdad = findFlambdad(p.CP[td02])
								ftl = findFTl(p.CP[td02])
							} else if ptSpread[rd02] >= ptSpread[t1k] &&
								ptSpread[rd02] >= ptSpread[mp] &&
								ptSpread[rd02] >= ptSpread[r1k] &&
								ptSpread[rd02] >= ptSpread[td02] {
								flambdad = findFlambdad(p.CP[rd02])
								ftl = findFTl(p.CP[rd02])
							}
						}
					}
				}

				probocc[n] = flambdad * ftl * fr * fs
			}

			// The biggest probocc.
			biggest = 0.0
			for n := 0; n < maxMds; n++ {
				if probocc[n] > biggest {
					biggest = probocc[n]
				}
			}

			p.probocc = biggest

			// The overall circuit reliability with scattering.
			p.OCRs = p.BCR * p.MIR * (100.0 - p.probocc) / 10000.0
		} else {
			p.OCRs = p.BCR * p.MIR / 100.0
		}
	}
}

// findFlambdad determines F sub lambda sub d of Appendix 1 to Annex 1 of P.533
// "A model for scattering of HF signals". The C code compares the dip in
// radians with limits in degrees converted by D2R but then uses the radian
// value in the formula.
func findFlambdad(cp controlPt) float64 {
	// The magnetic dip parameter.
	lambdad := math.Abs(cp.dip[hr100km])
	if 0.0 <= lambdad && lambdad < 15.0*d2r {
		return 1.0
	} else if 15.0*d2r <= lambdad && lambdad < 25.0*d2r {
		return math.Pow(((25.0-lambdad)/10.0), 2) * ((lambdad - 10.0) / 5.0)
	} else if 25.0*d2r <= lambdad && lambdad <= 90.0*d2r {
		return 0.0
	}

	return 0.0
}

// findFTl determines the time parameter F sub T sub l of Appendix 1 to Annex 1
// of P.533 for the calculation of Prob sub occ.
func findFTl(cp controlPt) float64 {
	tl := cp.ltime
	if 0.0 < tl && tl <= 3.0 {
		return 1.0
	} else if 3.0 < tl && tl <= 7.0 {
		return math.Pow(((7.0-tl)/4.0), 2) * ((tl - 1.0) / 2.0)
	} else if 7.0 < tl && tl <= 19.0 {
		return 0.0
	} else if 19.0 < tl && tl <= 20.0 {
		return math.Pow((tl-19.0), 2) * (41.0 - 2.0*tl)
	} else if 20.0 < tl && tl <= 24.0 {
		return 1.0
	}

	return 0.0
}
