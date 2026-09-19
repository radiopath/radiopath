package itm

import (
	"math"
	"sort"
)

func computeDeltaH(pfl []float64, d_start__meter, d_end__meter float64) float64 {
	np := int(pfl[0])
	x_start := d_start__meter / pfl[1]
	x_end := d_end__meter / pfl[1]

	if x_end-x_start < 2.0 {
		return 0
	}

	p10 := int(0.1 * (x_end - x_start + 8.0))
	p10 = min(max(4, p10), 25)

	n := 10*p10 - 5
	p90 := n - p10

	np_s := float64(n - 1)
	s := make([]float64, n+2)
	s[0] = np_s
	s[1] = 1.0

	x_end = (x_end - x_start) / np_s
	i := int(x_start)
	x_start -= float64(i + 1)

	for j := 0; j < n; j++ {
		for x_start > 0.0 && (i+1) < np {
			x_start--
			i++
		}

		s[j+2] = pfl[i+3] + (pfl[i+3]-pfl[i+2])*x_start

		x_start += x_end
	}

	var fit_y1, fit_y2 float64
	linearLeastSquaresFit(s, 0.0, np_s, &fit_y1, &fit_y2)

	fit_y2 = (fit_y2 - fit_y1) / np_s

	diffs := make([]float64, 0, n)
	for j := 0; j < n; j++ {
		diffs = append(diffs, s[j+2]-fit_y1)
		fit_y1 += fit_y2
	}

	sort.Sort(sort.Reverse(sort.Float64Slice(diffs)))
	q10 := diffs[p10-1]
	q90 := diffs[p90]

	delta_h_d__meter := q10 - q90

	// [ERL 79-ITS 67, Eqn 3], inverted
	delta_h__meter := delta_h_d__meter / (1.0 - 0.8*math.Exp(-(d_end__meter-d_start__meter)/50e3))

	return delta_h__meter
}
