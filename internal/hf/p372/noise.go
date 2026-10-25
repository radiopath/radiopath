package p372

import "math"

// Man-made noise environment categories (see ITU-R P.372). Any other value of
// Noise.ManMadeNoise is a user supplied noise figure.
const (
	City         = 0.0
	Residential  = 1.0
	Rural        = 2.0
	QuietRural   = 3.0
	Noisy        = 4.0
	Quiet        = 5.0
	d2r          = 0.0174532925 // Same truncated value as D2R in Common.h
	upperDecileK = 1.282
)

// Noise holds the inputs and outputs of the P.372 noise calculation. It
// mirrors struct NoiseParams of the C code, minus the coefficient arrays that
// live in Coefficients.
type Noise struct {
	// Hour is the UTC hour exactly as P533 passes path->hour to Noise():
	// 0-based, 0..23 (index 0 is UTC 01:00 in the P533 convention). It is
	// only used to find the receiver local time together with Lng.
	Hour int
	// Lng is the receiver longitude in radians, east positive.
	Lng float64
	// Lat is the receiver latitude in radians, north positive.
	Lat float64
	// Frequency is the frequency in MHz.
	Frequency float64
	// ManMadeNoise is the environment category (City, Residential, Rural,
	// QuietRural, Noisy, Quiet), a user noise figure, or a negative value
	// to override the whole calculation (FamT = -ManMadeNoise).
	ManMadeNoise float64

	FaA float64 // Atmospheric noise
	DuA float64 // Atmospheric noise upper decile
	DlA float64 // Atmospheric noise lower decile
	FaM float64 // Man-made noise
	DuM float64 // Man-made noise upper decile
	DlM float64 // Man-made noise lower decile
	FaG float64 // Galactic noise
	DuG float64 // Galactic noise upper decile
	DlG float64 // Galactic noise lower decile
	DuT float64 // Total noise upper decile
	DlT float64 // Total noise lower decile
	// FamT is the total (worst case) noise.
	FamT float64
}

// FamStats holds the statistics of the atmospheric noise Fam (struct FamStats).
type FamStats struct {
	Tmblk    int     // Time block
	FA       float64 // Atmospheric noise in dB above kT0b at 1 MHz
	SigmaFam float64 // Standard deviation of values, Fam
	Du       float64 // Ratio of upper decile to median value, Fam
	SigmaDu  float64 // Standard deviation of values of Du
	Dl       float64 // Ratio of median value, Fam, to lower decile
	SigmaDl  float64 // Standard deviation of values of Dl
}

// Compute determines the atmospheric, man-made and galactic noise, their
// decile values and the combined noise (Noise() in the C code). It does not
// modify c and keeps no state, so it is safe for concurrent use.
func Compute(n *Noise, c *Coefficients) error {
	if c == nil {
		return ErrNoCoefficients
	}

	// A negative man-made value cancels the calculation: there are no decile
	// values or noise components, only the total noise.
	if n.ManMadeNoise < 0.0 {
		n.DuA = 0.0
		n.DuG = 0.0
		n.DuM = 0.0

		n.DlA = 0.0
		n.DlG = 0.0
		n.DlM = 0.0

		n.FaA = 0.0
		n.FaG = 0.0
		n.FaM = n.ManMadeNoise

		n.DuT = 0.0
		n.DlT = 0.0

		n.FamT = -n.ManMadeNoise
		return nil
	}

	atmosphericNoise(n, c)
	galacticNoise(n)
	manMadeNoise(n)

	// Combination of noises from several sources, ITU-R P.372 section 8.
	// Upper decile first.
	sigmaA := n.DuA / upperDecileK
	sigmaG := 1.56
	sigmaM := n.DuM / upperDecileK
	famTu, sigmaT := combineNoise(n, sigmaA, sigmaG, sigmaM, n.DuA, n.DuG, n.DuM)
	n.DuT = upperDecileK * sigmaT

	// Lower decile.
	sigmaA = n.DlA / upperDecileK
	sigmaG = 1.56
	sigmaM = n.DlM / upperDecileK
	famTl, sigmaT := combineNoise(n, sigmaA, sigmaG, sigmaM, n.DlA, n.DlG, n.DlM)
	n.DlT = upperDecileK * sigmaT

	// Worst-case noise.
	if famTu < famTl {
		n.FamT = famTu
	} else {
		n.FamT = famTl
	}
	return nil
}

// combineNoise returns the combined noise and its sigma for one decile side.
// dA, dG and dM are the decile values that select the approximation.
func combineNoise(n *Noise, sigmaA, sigmaG, sigmaM, dA, dG, dM float64) (famT, sigmaT float64) {
	c := 10.0 / math.Log(10.0)

	alphaT := math.Exp((n.FaA/c)+(math.Pow(sigmaA, 2)/(2.0*math.Pow(c, 2)))) +
		math.Exp((n.FaG/c)+(math.Pow(sigmaG, 2)/(2.0*math.Pow(c, 2)))) +
		math.Exp((n.FaM/c)+(math.Pow(sigmaM, 2)/(2.0*math.Pow(c, 2))))

	betaT := math.Pow(math.Exp((n.FaA/c)+(math.Pow(sigmaA, 2)/(2.0*math.Pow(c, 2)))), 2)*(math.Exp(math.Pow(sigmaA/c, 2))-1.0) +
		math.Pow(math.Exp((n.FaG/c)+(math.Pow(sigmaG, 2)/(2.0*math.Pow(c, 2)))), 2)*(math.Exp(math.Pow(sigmaG/c, 2))-1.0) +
		math.Pow(math.Exp((n.FaM/c)+(math.Pow(sigmaM, 2)/(2.0*math.Pow(c, 2)))), 2)*(math.Exp(math.Pow(sigmaM/c, 2))-1.0)

	gammaT := math.Exp(n.FaA/c) + math.Exp(n.FaG/c) + math.Exp(n.FaM/c)

	if dA > 12.0 || dG > 12.0 || dM > 12.0 {
		sigmaT = c * math.Sqrt(2.0*math.Log(alphaT/gammaT))
	} else {
		sigmaT = c * math.Sqrt(math.Log(1.0+(betaT/math.Pow(alphaT, 2))))
	}

	famT = c * (math.Log(alphaT) - (math.Pow(sigmaT, 2) / (2.0 * math.Pow(c, 2))))
	return famT, sigmaT
}

// atmosphericNoise finds FaA, DuA and DlA by interpolating, on the receiver
// local mean time, between the current and the adjacent four-hour time block
// (AtmosphericNoise() in the C code).
func atmosphericNoise(n *Noise, c *Coefficients) {
	// Local time (0-23) from the UTC hour and the longitude; int() truncates
	// toward zero like the C cast.
	lrxmt := n.Hour + int(n.Lng/(15.0*d2r))

	// Roll over the local time if necessary.
	if lrxmt < 0 {
		lrxmt += 24
	} else if lrxmt > 23 {
		lrxmt -= 24
	}

	var now, adj FamStats
	now.Tmblk = (lrxmt / 4) % 6
	adj.Tmblk = (now.Tmblk + 1) % 6

	getFamParameters(c, &now, n.Lng, n.Lat, n.Frequency)
	getFamParameters(c, &adj, n.Lng, n.Lat, n.Frequency)

	slp := math.Mod(float64(lrxmt), 4.0) / 4.0

	n.FaA = interpolateDB(now.FA, adj.FA, slp)
	n.DuA = interpolateDB(now.Du, adj.Du, slp)
	n.DlA = interpolateDB(now.Dl, adj.Dl, slp)
}

// interpolateDB interpolates linearly in the power domain between two values
// given in dB and returns the result in dB.
func interpolateDB(now, adj, slp float64) float64 {
	fa := math.Pow(10.0, (now/10.0)) +
		(math.Pow(10.0, (adj/10.0))-math.Pow(10.0, (now/10.0)))*slp
	return 10.0 * math.Log10(fa)
}

// getFamParameters finds the atmospheric noise parameters of Figs. 15c to 38c
// in ITU-R P.372 for the time block fs.Tmblk (GetFamParameters() in the C code).
func getFamParameters(c *Coefficients, fs *FamStats, lng, lat, frequency float64) {
	var v [5]float64
	var u [2]float64
	var zz [29]float64
	cz := 0.0
	var pz, px float64

	const lm = 29
	const ln = 15

	// The series uses the geographic east longitude (0 to 2*pi) halved.
	var q float64
	if lng < 0.0 {
		q = (lng + 2.0*math.Pi) / 2.0
	} else {
		q = lng / 2.0
	}

	// Longitude series.
	for j := 0; j < lm; j++ {
		r := 0.0
		for k := 0; k < ln; k++ {
			r = r + math.Sin((float64(k)+1.0)*q)*c.Fakp[fs.Tmblk][k][j]
		}
		zz[j] = r + c.Fakp[fs.Tmblk][15][j]
	}

	// Latitude series, with q reused as the latitude plus 90 degrees.
	q = lat + math.Pi/2.0

	r := 0.0
	for j := 0; j < lm; j++ {
		r = r + math.Sin((float64(j)+1.0)*q)*zz[j]
	}
	// Linear normalisation with the Fakabp values.
	fam1MHz := r + c.Fakabp[fs.Tmblk][0] + c.Fakabp[fs.Tmblk][1]*q

	// Southern hemisphere uses the second set of six blocks.
	i := fs.Tmblk
	if lat < 0 {
		i = fs.Tmblk + 6
	}

	u[0] = -0.75
	u[1] = (8.0*math.Pow(2.0, math.Log10(frequency)) - 11.0) / 4.0

	// See NBS Tech Note 318, Lucas and Harper. After the loop pz and px
	// hold the values for u[1].
	for k := 0; k < 2; k++ {
		pz = u[k]*c.Fam[i][0] + c.Fam[i][1]
		px = u[k]*c.Fam[i][7] + c.Fam[i][8]

		for j := 2; j < 7; j++ {
			pz = u[k]*pz + c.Fam[i][j]
			px = u[k]*px + c.Fam[i][j+7]
		}

		if k == 0 {
			cz = fam1MHz*(2.0-pz) - px
		}
	}

	// Frequency variation of atmospheric noise.
	fs.FA = cz*pz + px

	// The P.372 curves for Du, Dl, SigmaDu and SigmaDl stop at 20 MHz and
	// those for SigmaFam at 10 MHz.
	x := math.Log10(frequency)
	if frequency > 20.0 {
		x = math.Log10(20.0)
	}

	for j := 0; j < 5; j++ {
		if j == 4 && frequency > 10.0 {
			x = 1.0
		}
		y := c.Dud[j][i][0]
		for k := 1; k < 5; k++ {
			y = y*x + c.Dud[j][i][k]
		}
		v[j] = y
	}

	fs.Du = v[0]
	fs.Dl = v[1]
	fs.SigmaDu = v[2]
	fs.SigmaDl = v[3]
	fs.SigmaFam = v[4]
}

// manMadeNoise determines FaM, DuM and DlM per ITU-R P.372 section 5.
func manMadeNoise(n *Noise) {
	var c, d float64

	switch n.ManMadeNoise {
	case City:
		c = 76.8
		d = 27.7
		n.DuM = 11.0
		n.DlM = 6.7
	case Residential:
		c = 72.5
		d = 27.7
		n.DuM = 10.6
		n.DlM = 5.3
	case Rural:
		c = 67.2
		d = 27.7
		n.DuM = 9.2
		n.DlM = 4.6
	case QuietRural:
		c = 53.6
		d = 28.6
		n.DuM = 9.2
		n.DlM = 4.6
	case Quiet: // Not in ITU-R P.372-10
		c = 65.2
		d = 29.1
		n.DuM = 9.2
		n.DlM = 4.6
	case Noisy: // Not in ITU-R P.372-10
		c = 83.2
		d = 37.5
		n.DuM = 11.0
		n.DlM = 6.7
	default:
		// User supplied value. The C code assigns the deciles swapped
		// (Dl = 11.0, Du = 6.7); kept as is to match its output.
		c = -n.ManMadeNoise + 204.0
		d = 0.0
		n.DlM = 11.0
		n.DuM = 6.7
	}

	n.FaM = c - d*math.Log10(n.Frequency)
}

// galacticNoise determines FaG, DuG and DlG per ITU-R P.372 table 1.
func galacticNoise(n *Noise) {
	c := 52.0
	d := 23.0
	n.FaG = c - d*math.Log10(n.Frequency)

	// Deciles are fixed at 2 dB (3/1.282).
	n.DuG = 2.0
	n.DlG = 2.0
}

// AtmosphericNoiseLT is the utility used to generate the atmospheric noise
// figures of Rec. P.372-14 (AtmosphericNoise_LT() in the C code). It takes the
// local time lrxmt (0-23) and returns the full statistics, with Tmblk set to
// 99 as an indicator.
func AtmosphericNoiseLT(c *Coefficients, lrxmt int, lng, lat, frequency float64) FamStats {
	if lrxmt < 0 {
		lrxmt += 24
	} else if lrxmt > 23 {
		lrxmt -= 24
	}

	var now, adj FamStats
	now.Tmblk = (lrxmt / 4) % 6
	adj.Tmblk = (now.Tmblk + 1) % 6

	getFamParameters(c, &now, lng, lat, frequency)
	getFamParameters(c, &adj, lng, lat, frequency)

	slp := math.Mod(float64(lrxmt), 4.0) / 4.0

	return FamStats{
		Tmblk:    99,
		FA:       interpolateDB(now.FA, adj.FA, slp),
		Du:       interpolateDB(now.Du, adj.Du, slp),
		Dl:       interpolateDB(now.Dl, adj.Dl, slp),
		SigmaDl:  interpolateDB(now.SigmaDl, adj.SigmaDl, slp),
		SigmaDu:  interpolateDB(now.SigmaDu, adj.SigmaDu, slp),
		SigmaFam: interpolateDB(now.SigmaFam, adj.SigmaFam, slp),
	}
}
