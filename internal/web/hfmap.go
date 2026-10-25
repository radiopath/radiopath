package web

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"runtime"
	"sync"

	"github.com/danielgtaylor/huma/v2"

	"github.com/radiopath/radiopath/internal/hf"
	"github.com/radiopath/radiopath/internal/store"
)

const (
	// hfMapStep is the edge of a cell of the map in degrees; a map is 648 cells at 10, each one run of the model per hour.
	hfMapStep = 10.0
	// hfMapUserMax is the number of maps per user and minute: one is up to 15000 runs, a path at most 720.
	hfMapUserMax = 6
)

// errClientGone answers a request whose client hung up: 499 as in nginx, so it is neither a server error in the
// metrics nor in the log; nobody reads the body.
var errClientGone = huma.NewError(499, "Client closed request")

type HFMapInput struct {
	SiteID int64 `json:"site_id" doc:"Transmitting site; the map shows the circuit from it to every cell of the globe"`

	Year         int     `json:"year" doc:"Year, 1900 to 2100"`
	Month        int     `json:"month" doc:"Month, 1 to 12"`
	HoursUTC     []int   `json:"hours_utc" doc:"UTC hours, 1 to 24, at most 24 values; the map has one frame per hour"`
	SSN          int     `json:"ssn" doc:"12-month smoothed sunspot number R12, 1 to 311"`
	FrequencyMHz float64 `json:"frequency_mhz" doc:"Frequency, 1 to 30 MHz"`

	TxPowerDBW       float64    `json:"tx_power_dbw" doc:"Transmitter power in dB(1 kW), -30 to 60"`
	TxGainDBi        float64    `json:"tx_gain_dbi,omitempty" doc:"Gain of the isotropic antenna at the site"`
	RxGainDBi        float64    `json:"rx_gain_dbi,omitempty" doc:"Gain of the isotropic antenna at every cell"`
	BandwidthHz      float64    `json:"bandwidth_hz" doc:"0.005 Hz to 3 MHz"`
	RequiredSNRDB    float64    `json:"required_snr_db" doc:"Required signal-to-noise ratio, -30 to 200"`
	SNRPercentile    int        `json:"snr_percentile" doc:"Share of the month, 1 to 99"`
	NoiseEnvironment string     `json:"noise_environment" enum:"city,residential,rural,quiet_rural,noisy,quiet" doc:"Man-made noise environment, ITU-R P.372"`
	Modulation       string     `json:"modulation,omitempty" enum:"analog,digital" doc:"analog without a value"`
	Digital          *HFDigital `json:"digital,omitempty" doc:"Digital modulation parameters; without them the defaults of the ITU program apply (required SIR 10 dB, all others 0)"`
	LongPath         bool       `json:"long_path,omitempty" doc:"The long way round the globe instead of the short path"`
}

type HFMapFrame struct {
	HourUTC        int   `json:"hour_utc"`
	ReliabilityPct []int `json:"reliability_pct" doc:"Circuit reliability in percent per cell, rows from north to south and cells from west to east; -307 where the model could not calculate the cell"`
}

type HFMap struct {
	StepDeg float64      `json:"step_deg" doc:"Edge of a cell in degrees; the first cell is centred half a step south of 90 N and east of 180 W"`
	Rows    int          `json:"rows"`
	Cols    int          `json:"cols"`
	Frames  []HFMapFrame `json:"frames" doc:"One frame per requested hour, in the order of the request"`
}

type HFMapOutput struct{ Body HFMap }

// hfMapRequest validates a map input and returns the model request for the circuit from site; the receiver is set per
// cell.
func hfMapRequest(in HFMapInput, site store.Site) (hf.Request, error) {
	var c check
	req := hfRequest(&c, HFPredictionInput{
		Year: in.Year, Month: in.Month, HoursUTC: in.HoursUTC, SSN: in.SSN,
		FrequenciesMHz: []float64{in.FrequencyMHz}, TxPowerDBW: in.TxPowerDBW, TxGainDBi: in.TxGainDBi, RxGainDBi: in.RxGainDBi,
		BandwidthHz: in.BandwidthHz, RequiredSNRDB: in.RequiredSNRDB, SNRPercentile: in.SNRPercentile,
		NoiseEnvironment: in.NoiseEnvironment, Modulation: in.Modulation, Digital: in.Digital, LongPath: in.LongPath,
	}, site, "frequency_mhz")
	return req, c.err()
}

// hfCellCentre returns the centre of the cell in a row (from the north) and a column (from the west) of the map.
func hfCellCentre(row, col int) (lat, lon float64) {
	return 90 - hfMapStep*(float64(row)+0.5), -180 + hfMapStep*(float64(col)+0.5)
}

// hfReliability is the circuit reliability to report for a point: the overall one exists for digital modulation up to
// 9000 km, otherwise the model leaves it at 0 and the basic one is the figure.
func hfReliability(p hf.Point, digital bool) float64 {
	if digital && p.Distance <= 9000 {
		return p.OCR
	}
	return p.BCR
}

// hfMapCompute runs the model for the circuit req from its transmitter to the centre of every cell of the globe on all
// CPUs; cells the model cannot calculate stay at hfNotAvailable, and it fails only when none could be calculated. It
// runs in a worker; the short-job pool bounds how many maps a process computes at once.
func hfMapCompute(ctx context.Context, data *hf.Data, req hf.Request) (HFMap, error) {
	rows, cols := int(180/hfMapStep), int(360/hfMapStep)
	digital := req.Modulation == hf.Digital
	frames := make([]HFMapFrame, len(req.Hours))
	for i, h := range req.Hours {
		frames[i] = HFMapFrame{HourUTC: h, ReliabilityPct: make([]int, rows*cols)}
	}

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		failed   int
		firstErr error
	)
	cells := make(chan int)
	for w := 0; w < runtime.GOMAXPROCS(0); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range cells {
				r := req
				r.RxLat, r.RxLng = hfCellCentre(c/cols, c%cols)
				res, err := data.Predict(r)
				if err != nil {
					mu.Lock()
					failed++
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					for i := range frames {
						frames[i].ReliabilityPct[c] = hfNotAvailable
					}
					continue
				}
				for i, p := range res.Points {
					frames[i].ReliabilityPct[c] = int(math.Round(finite(hfReliability(p, digital))))
				}
			}
		}()
	}
feed:
	for c := 0; c < rows*cols; c++ {
		select {
		case cells <- c:
		case <-ctx.Done():
			break feed
		}
	}
	close(cells)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return HFMap{}, err
	}
	if failed == rows*cols {
		return HFMap{}, firstErr
	}
	return HFMap{StepDeg: hfMapStep, Rows: rows, Cols: cols, Frames: frames}, nil
}

// hfMapHit counts one map of user and answers 429 over hfMapUserMax per minute.
func (s *Server) hfMapHit(ctx context.Context, user int64) error {
	n, err := s.Store.RateHit(ctx, fmt.Sprintf("hfmap:user:%d", user), computeWindow)
	if err != nil {
		return s.storeErr(err)
	}
	if n > hfMapUserMax {
		s.Log.Warn("hf map rate limited", "user", user)
		return huma.Error429TooManyRequests("Too many maps, try again in a minute.")
	}
	return nil
}

// registerHFMap registers the HF reliability map (ITU-R P.533) from one of the user's sites to the whole globe.
func (s *Server) registerHFMap(api huma.API) {
	o := op("predict-hf-map", http.MethodPost, "/api/hf/maps", scopeHFRead,
		"HF reliability map from a site to every cell of the globe (ITU-R P.533), nothing is stored")
	o.Errors = append(o.Errors, http.StatusUnprocessableEntity, http.StatusTooManyRequests, http.StatusServiceUnavailable)
	huma.Register(api, o, func(ctx context.Context, in *struct{ Body HFMapInput }) (*HFMapOutput, error) {
		owner := userOf(ctx).ID
		if err := s.hfMapHit(ctx, owner); err != nil {
			return nil, err
		}
		site, err := s.Store.GetSite(ctx, owner, in.Body.SiteID)
		if err != nil {
			return nil, s.storeErr(err)
		}
		req, err := hfMapRequest(in.Body, site)
		if err != nil {
			return nil, err
		}
		var m HFMap
		if err := s.hfRun(ctx, owner, kindHFMap, hfJob{Request: req}, &m); err != nil {
			return nil, err
		}
		return &HFMapOutput{m}, nil
	})
}
