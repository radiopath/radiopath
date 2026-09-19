package link

import (
	"math"

	"github.com/radiopath/radiopath/internal/itm"
)

// ITU-R P.833-10 §2.1, terminal in woodland: A = A_m (1 - exp(-d gamma / A_m))

func gammaDBm(freqMHz float64, pol itm.Polarization) float64 {
	f := freqMHz / 1000
	if f < 1 && pol == itm.Vertical {
		return 0.2 * math.Pow(f, 0.66)
	}
	return 0.2 * f
}

func maxAttenuationDB(freqMHz float64) float64 { return 1.37 * math.Pow(freqMHz, 0.42) }

func excessDB(dM, freqMHz float64, pol itm.Polarization) float64 {
	if dM <= 0 {
		return 0
	}
	am := maxAttenuationDB(freqMHz)
	return am * (1 - math.Exp(-dM*gammaDBm(freqMHz, pol)/am))
}

func canopyPath(p []Sample) (dA, dB float64) {
	n := len(p)
	gap := func(i int) float64 { return p[i].RayM - p[i].TerrainM - p[i].BulgeM - p[i].CanopyM }
	d := p[n-1].DistM
	clear := false
	for i := 0; i < n && !clear; i++ {
		clear = gap(i) >= 0
	}
	if !clear {
		return d, 0
	}
	mid := d / 2
	for i := 1; i < n; i++ {
		a, b := gap(i-1), gap(i)
		x0, x1 := p[i-1].DistM, p[i].DistM
		switch {
		case a < 0 && b < 0:
		case a < 0:
			x1 = x0 + a/(a-b)*(x1-x0)
		case b < 0:
			x0 = x0 + a/(a-b)*(x1-x0)
		default:
			continue
		}
		dA += max(0, min(x1, mid)-x0)
		dB += max(0, x1-max(x0, mid))
	}
	return dA, dB
}
