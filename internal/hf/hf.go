package hf

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/radiopath/radiopath/internal/hf/p372"
)

// Data holds the data files of the model. The decile factors are read once by
// Open; the ionospheric maps (11 MB per month) and the noise coefficients are
// loaded lazily per month and cached. A Data value has no mutable state beyond
// that cache, so Predict may be called concurrently.
type Data struct {
	p533Dir string
	p372Dir string
	foF2var *decileFactors

	months [12]monthData
}

// monthData is the cached, read-only data of one month; coeff is nil until it is loaded.
type monthData struct {
	mu    sync.Mutex
	foF2  *ionMap
	m3k   *ionMap
	coeff *p372.Coefficients
}

// Open prepares the data of the model. p533Dir holds ionosNN.bin and
// 'P1239-3 Decile Factors.txt', p372Dir the COEFFNNW.txt files of the P.372
// noise model; an empty p372Dir means the same directory as p533Dir (the
// layout of the ITU data set).
func Open(p533Dir, p372Dir string) (*Data, error) {
	if p372Dir == "" {
		p372Dir = p533Dir
	}
	f, err := readP1239(p533Dir)
	if err != nil {
		return nil, err
	}
	return &Data{p533Dir: p533Dir, p372Dir: p372Dir, foF2var: f}, nil
}

// Ready reports whether the data files are still there, which they are not when a mounted volume goes away after
// startup; it only looks at the directory entries, not at the content.
func (d *Data) Ready() error {
	files := []string{filepath.Join(d.p533Dir, "P1239-3 Decile Factors.txt")}
	for m := 1; m <= 12; m++ {
		files = append(files,
			filepath.Join(d.p533Dir, fmt.Sprintf("ionos%02d.bin", m)),
			filepath.Join(d.p372Dir, fmt.Sprintf("COEFF%02dW.txt", m)))
	}
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			return err
		}
	}
	return nil
}

// month returns the data of a month (0-11), loading it on first use. Only a complete load is kept, so a failed read
// is tried again by the next call instead of failing that month until a restart.
func (d *Data) month(m int) (*monthData, error) {
	md := &d.months[m]
	md.mu.Lock()
	defer md.mu.Unlock()
	if md.coeff != nil {
		return md, nil
	}
	foF2, m3k, err := readIonParametersBin(m, d.p533Dir)
	if err != nil {
		return nil, err
	}
	coeff, err := p372.LoadCoefficients(d.p372Dir, m)
	if err != nil {
		return nil, err
	}
	md.foF2, md.m3k, md.coeff = foF2, m3k, coeff
	return md, nil
}

// Predict runs the model for every hour and frequency of the request, hours in
// the outer loop as in the C driver.
func (d *Data) Predict(req Request) (*Result, error) {
	if err := req.validate(); err != nil {
		return nil, err
	}
	md, err := d.month(req.Month - 1)
	if err != nil {
		return nil, fmt.Errorf("hf: month %d: %w", req.Month, err)
	}
	p, err := req.newPath()
	if err != nil {
		return nil, err
	}
	p.foF2 = md.foF2
	p.M3kF2 = md.m3k
	p.coeff = md.coeff
	p.foF2var = d.foF2var

	res := &Result{Points: make([]Point, 0, len(req.Hours)*len(req.Frequencies))}
	// The path is reused for all runs, like the PathData of the C driver.
	for _, h := range req.Hours {
		p.hour = h - 1
		for _, f := range req.Frequencies {
			p.frequency = f
			if err := calculate(p); err != nil {
				return nil, err
			}
			res.Points = append(res.Points, newPoint(p))
		}
	}
	return res, nil
}
