package tiles

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
)

const tileSize = 256

type View struct {
	Zoom   int
	X0, Y0 float64
	W, H   int
}

func worldPixel(lat, lon float64, zoom int) (x, y float64) {
	n := float64(tileSize) * math.Exp2(float64(zoom))
	phi := lat * math.Pi / 180
	return (lon + 180) / 360 * n, (1 - math.Log(math.Tan(phi)+1/math.Cos(phi))/math.Pi) / 2 * n
}

func Fit(south, west, north, east float64, w, h, pad int) View {
	for z := maxZoom; ; z-- {
		x1, y1 := worldPixel(north, west, z)
		x2, y2 := worldPixel(south, east, z)
		if z == 0 || (x2-x1 <= float64(w-2*pad) && y2-y1 <= float64(h-2*pad)) {
			return View{Zoom: z, X0: (x1+x2)/2 - float64(w)/2, Y0: (y1+y2)/2 - float64(h)/2, W: w, H: h}
		}
	}
}

func (v View) Pixel(lat, lon float64) (x, y float64) {
	x, y = worldPixel(lat, lon, v.Zoom)
	return x - v.X0, y - v.Y0
}

func (v View) Trim(south, west, north, east float64, pad int) View {
	x1, y1 := v.Pixel(north, west)
	x2, y2 := v.Pixel(south, east)
	w := min(v.W, int(math.Ceil(x2-x1))+2*pad)
	h := min(v.H, int(math.Ceil(y2-y1))+2*pad)
	return View{Zoom: v.Zoom, X0: v.X0 + (x1+x2)/2 - float64(w)/2, Y0: v.Y0 + (y1+y2)/2 - float64(h)/2, W: w, H: h}
}

func (v View) MetresPerPixel(lat float64) float64 {
	return 2 * math.Pi * 6378137 * math.Cos(lat*math.Pi/180) / (tileSize * math.Exp2(float64(v.Zoom)))
}

func (p *Proxy) Static(ctx context.Context, v View) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, v.W, v.H))
	n := 1 << v.Zoom
	x0, y0 := int(math.Floor(v.X0)), int(math.Floor(v.Y0))
	for ty := y0 / tileSize; ty*tileSize < y0+v.H; ty++ {
		if ty < 0 || ty >= n {
			continue
		}
		for tx := int(math.Floor(float64(x0) / tileSize)); tx*tileSize < x0+v.W; tx++ {
			raw, _, err := p.tile(ctx, "", v.Zoom, ((tx%n)+n)%n, ty)
			if err != nil {
				return nil, fmt.Errorf("tiles: %d/%d/%d: %w", v.Zoom, tx, ty, err)
			}
			t, err := png.Decode(bytes.NewReader(raw))
			if err != nil {
				return nil, fmt.Errorf("tiles: %d/%d/%d: %w", v.Zoom, tx, ty, err)
			}
			at := image.Pt(tx*tileSize-x0, ty*tileSize-y0)
			draw.Draw(img, t.Bounds().Add(at), t, image.Point{}, draw.Src)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("tiles: %w", err)
	}
	return buf.Bytes(), nil
}
