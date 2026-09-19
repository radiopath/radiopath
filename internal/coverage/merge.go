package coverage

import "math"

const maxMergePixels = 9_000_000

func Merge(rs []Result) Result {
	var in []Result
	for _, r := range rs {
		if r.W > 0 && r.H > 0 && len(r.Margin) == r.W*r.H {
			in = append(in, r)
		}
	}
	if len(in) == 0 {
		return Result{}
	}
	if len(in) == 1 {
		return in[0]
	}

	out := Result{North: in[0].North, South: in[0].South, East: in[0].East, West: in[0].West}
	dLat, dLon := math.Inf(1), math.Inf(1)
	for _, r := range in {
		out.North, out.South = math.Max(out.North, r.North), math.Min(out.South, r.South)
		out.East, out.West = math.Max(out.East, r.East), math.Min(out.West, r.West)
		dLat = math.Min(dLat, (r.North-r.South)/float64(r.H))
		dLon = math.Min(dLon, (r.East-r.West)/float64(r.W))
	}
	spanLat, spanLon := out.North-out.South, out.East-out.West
	w, h := int(math.Ceil(spanLon/dLon)), int(math.Ceil(spanLat/dLat))
	if w*h > maxMergePixels {
		f := math.Sqrt(float64(w*h) / maxMergePixels)
		w, h = int(math.Ceil(float64(w)/f)), int(math.Ceil(float64(h)/f))
	}
	out.W, out.H = w, h
	out.Margin = make([]float64, w*h)

	for y := 0; y < h; y++ {
		lat := out.North - (float64(y)+0.5)*spanLat/float64(h)
		for x := 0; x < w; x++ {
			lon := out.West + (float64(x)+0.5)*spanLon/float64(w)
			best := math.NaN()
			for _, r := range in {
				if lat > r.North || lat < r.South || lon < r.West || lon > r.East {
					continue
				}
				sx := min(int((lon-r.West)/(r.East-r.West)*float64(r.W)), r.W-1)
				sy := min(int((r.North-lat)/(r.North-r.South)*float64(r.H)), r.H-1)
				if v := r.At(sx, sy); !math.IsNaN(v) && (math.IsNaN(best) || v > best) {
					best = v
				}
			}
			out.Margin[y*w+x] = best
		}
	}
	return out
}
