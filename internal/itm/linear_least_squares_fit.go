package itm

func linearLeastSquaresFit(pfl []float64, d_start, d_end float64, fit_y1, fit_y2 *float64) {
	np := int(pfl[0])

	i_start := int(fdim(d_start/pfl[1], 0.0))
	i_end := np - int(fdim(float64(np), d_end/pfl[1]))

	if i_end <= i_start {
		i_start = int(fdim(float64(i_start), 1.0))
		i_end = np - int(fdim(float64(np), float64(i_end)+1.0))
	}

	x_length := float64(i_end - i_start)

	mid_shifted_index := -0.5 * x_length
	mid_shifted_end := float64(i_end) + mid_shifted_index

	sum_y := 0.5 * (pfl[i_start+2] + pfl[i_end+2])
	scaled_sum_y := 0.5 * (pfl[i_start+2] - pfl[i_end+2]) * mid_shifted_index

	for i := 2; float64(i) <= x_length; i++ {
		i_start++
		mid_shifted_index++

		sum_y += pfl[i_start+2]
		scaled_sum_y += pfl[i_start+2] * mid_shifted_index
	}

	sum_y = sum_y / x_length
	scaled_sum_y = scaled_sum_y * 12.0 / ((x_length*x_length + 2.0) * x_length)

	*fit_y1 = sum_y - scaled_sum_y*mid_shifted_end
	*fit_y2 = sum_y + scaled_sum_y*(float64(np)-mid_shifted_end)
}
