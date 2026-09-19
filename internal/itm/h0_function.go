package itm

import "math"

func h0Curve(j int, r float64) float64 {
	a := [...]float64{25.0, 80.0, 177.0, 395.0, 705.0}
	b := [...]float64{24.0, 45.0, 68.0, 80.0, 105.0}

	return 10 * math.Log10(1+a[j]*math.Pow(1/r, 4)+b[j]*math.Pow(1.0/r, 2)) // related to TN101v2, Eqn III.49, but from [Algorithm, 6.13]
}

// troposcatter frequency gain H_0 [TN101v1, Ch 9.2]
func h0Function(r, eta_s float64) float64 {
	eta_s = math.Min(math.Max(eta_s, 1), 5)

	i := int(eta_s)
	q := eta_s - float64(i)

	result := h0Curve(i-1, r)

	if q != 0.0 {
		result = (1.0-q)*result + q*h0Curve(i, r)
	}

	return result
}
