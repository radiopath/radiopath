package web

import (
	"context"
	"math"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/radiopath/radiopath/internal/antenna"
	"github.com/radiopath/radiopath/internal/hf"
	"github.com/radiopath/radiopath/internal/store"
)

const scopeHFRead = "hf:read"

// Limits of one prediction: every hour and frequency is one run of the model.
const (
	maxHFHours = 24
	maxHFFreqs = 30
)

// hfNotAvailable is the value the model reports for quantities that do not apply, for example the short path field
// strength of a path longer than 9000 km.
const hfNotAvailable = -307

// hfNoise maps the API names of the man-made noise environments (ITU-R P.372) to the model's.
var hfNoise = map[string]float64{
	"city": hf.NoiseCity, "residential": hf.NoiseResidential, "rural": hf.NoiseRural,
	"quiet_rural": hf.NoiseQuietRural, "noisy": hf.NoiseNoisy, "quiet": hf.NoiseQuiet,
}

type HFDigital struct {
	RequiredSIRDB float64 `json:"required_sir_db" doc:"Required signal-to-interference ratio"`
	F0Hz          float64 `json:"f0_hz" doc:"Frequency dispersion at -10 dB of the peak"`
	T0Ms          float64 `json:"t0_ms" doc:"Time spread at -10 dB of the peak"`
	ADB           float64 `json:"a_db" doc:"Required amplitude ratio"`
	TWMs          float64 `json:"tw_ms" doc:"Time window"`
	FWHz          float64 `json:"fw_hz" doc:"Frequency window"`
}

type HFPredictionInput struct {
	SiteAID int64 `json:"site_a_id" doc:"Transmitting site"`
	SiteBID int64 `json:"site_b_id" doc:"Receiving site"`

	Year           int       `json:"year" doc:"Year, 1900 to 2100"`
	Month          int       `json:"month" doc:"Month, 1 to 12"`
	HoursUTC       []int     `json:"hours_utc" doc:"UTC hours, 1 to 24, at most 24 values"`
	SSN            int       `json:"ssn" doc:"12-month smoothed sunspot number R12, 1 to 311"`
	FrequenciesMHz []float64 `json:"frequencies_mhz" doc:"Frequencies, 1 to 30 MHz, at most 30 values"`

	TxPowerDBW       float64    `json:"tx_power_dbw" doc:"Transmitter power in dB(1 kW), -30 to 60"`
	TxGainDBi        float64    `json:"tx_gain_dbi,omitempty" doc:"Gain of an isotropic antenna at A; ignored with tx_antenna_id"`
	RxGainDBi        float64    `json:"rx_gain_dbi,omitempty" doc:"Gain of an isotropic antenna at B; ignored with rx_antenna_id"`
	TxAntennaID      *int64     `json:"tx_antenna_id,omitempty" doc:"Antenna at A: its gain and its horizontal and vertical pattern (elevation above the horizon) are used; isotropic without"`
	TxAzimuthDeg     *float64   `json:"tx_azimuth_deg,omitempty" doc:"Main lobe azimuth at A; without it the antenna aims at B"`
	RxAntennaID      *int64     `json:"rx_antenna_id,omitempty" doc:"Antenna at B, see tx_antenna_id"`
	RxAzimuthDeg     *float64   `json:"rx_azimuth_deg,omitempty" doc:"Main lobe azimuth at B; without it the antenna aims at A"`
	BandwidthHz      float64    `json:"bandwidth_hz" doc:"0.005 Hz to 3 MHz"`
	RequiredSNRDB    float64    `json:"required_snr_db" doc:"Required signal-to-noise ratio, -30 to 200"`
	SNRPercentile    int        `json:"snr_percentile" doc:"Share of the month, 1 to 99, for which snr_percentile_db is exceeded"`
	NoiseEnvironment string     `json:"noise_environment" enum:"city,residential,rural,quiet_rural,noisy,quiet" doc:"Man-made noise environment, ITU-R P.372"`
	Modulation       string     `json:"modulation,omitempty" enum:"analog,digital" doc:"analog without a value"`
	Digital          *HFDigital `json:"digital,omitempty" doc:"Digital modulation parameters; without them the defaults of the ITU program apply (required SIR 10 dB, all others 0)"`
	LongPath         bool       `json:"long_path,omitempty" doc:"The long way round the globe instead of the short path"`
}

type HFPoint struct {
	Month        int     `json:"month"`
	HourUTC      int     `json:"hour_utc"`
	FrequencyMHz float64 `json:"frequency_mhz"`

	DistanceKm float64 `json:"distance_km"`
	MUFBasic   float64 `json:"muf_basic_mhz" doc:"Basic MUF"`
	MUF10      float64 `json:"muf_10_mhz" doc:"MUF exceeded on 10% of the days of the month"`
	MUF50      float64 `json:"muf_50_mhz" doc:"MUF exceeded on 50% of the days of the month"`
	MUF90      float64 `json:"muf_90_mhz" doc:"MUF exceeded on 90% of the days of the month"`
	MUFOper    float64 `json:"muf_operational_mhz" doc:"Operational MUF"`

	FieldStrengthDBuV float64 `json:"field_strength_dbuv_m" doc:"Median field strength, dB(1 uV/m)"`
	RxPowerDBW        float64 `json:"rx_power_dbw" doc:"Median available receiver power"`
	NoiseDB           float64 `json:"noise_db" doc:"Total noise, dB above kT0b"`
	SNRDB             float64 `json:"snr_db" doc:"Median signal-to-noise ratio"`
	SNRPercentileDB   float64 `json:"snr_percentile_db" doc:"Signal-to-noise ratio exceeded for snr_percentile of the month"`

	BasicReliabilityPct   float64 `json:"basic_reliability_pct" doc:"Basic circuit reliability"`
	OverallReliabilityPct float64 `json:"overall_reliability_pct" doc:"Overall circuit reliability"`

	DominantMode          string  `json:"dominant_mode" doc:"For example 2F2, or NONE"`
	DominantElevationDeg  float64 `json:"dominant_elevation_deg"`
	DominantDelayMs       float64 `json:"dominant_delay_ms"`
	DominantLossDB        float64 `json:"dominant_loss_db" doc:"Basic transmission loss of the dominant mode"`
	DominantSupportedPct  float64 `json:"dominant_supported_pct" doc:"Probability that the dominant mode is supported at the frequency"`
	DominantReflectionKm  float64 `json:"dominant_reflection_height_km"`
	DominantFieldStrength float64 `json:"dominant_field_strength_dbuv_m"`
}

type HFPrediction struct {
	Points []HFPoint `json:"points" doc:"One point per hour and frequency, ordered by the requested hours, then frequencies. A value of -307 means the quantity does not apply to the path"`
}

type HFPredictionOutput struct{ Body HFPrediction }

// parseHF validates a prediction input and returns the model request for the sites a and b.
func parseHF(in HFPredictionInput, a, b store.Site) (hf.Request, error) {
	var c check
	req := hfRequest(&c, in, a, "frequencies_mhz")
	req.RxLat, req.RxLng = b.Lat, b.Lon
	if a.ID == b.ID {
		c.add("", "Site A and B must differ")
	}
	return req, c.err()
}

// hfRequest checks the fields a prediction and a map share into c and returns the model request from the site tx,
// without a receiver; freqField is the name of the frequency field in the caller's input.
func hfRequest(c *check, in HFPredictionInput, tx store.Site, freqField string) hf.Request {
	req := hf.Request{
		TxLat: tx.Lat, TxLng: tx.Lon,
		Year:         int(c.num("year", "Year", float64(in.Year), 1900, 2100)),
		Month:        int(c.num("month", "Month", float64(in.Month), 1, 12)),
		SSN:          int(c.num("ssn", "Sunspot number", float64(in.SSN), 1, 311)),
		TxPower:      c.num("tx_power_dbw", "TX power", in.TxPowerDBW, -30, 60),
		Bandwidth:    c.num("bandwidth_hz", "Bandwidth", in.BandwidthHz, 0.005, 3e6),
		RequiredSNR:  c.num("required_snr_db", "Required SNR", in.RequiredSNRDB, -30, 200),
		SNRXXPercent: int(c.num("snr_percentile", "SNR percentile", float64(in.SNRPercentile), 1, 99)),
		LongPath:     in.LongPath,
	}
	// A stored antenna brings its own gain, the isotropic one is then ignored as documented.
	if in.TxAntennaID == nil {
		req.TxAntenna.GainOffset = c.num("tx_gain_dbi", "TX gain", in.TxGainDBi, -50, 60)
	}
	if in.RxAntennaID == nil {
		req.RxAntenna.GainOffset = c.num("rx_gain_dbi", "RX gain", in.RxGainDBi, -50, 60)
	}
	req.TxAntenna.Bearing = c.deg("tx_azimuth_deg", "TX azimuth", in.TxAzimuthDeg)
	req.RxAntenna.Bearing = c.deg("rx_azimuth_deg", "RX azimuth", in.RxAzimuthDeg)

	if len(in.HoursUTC) == 0 || len(in.HoursUTC) > maxHFHours {
		c.add("hours_utc", "Hours: between 1 and 24 values")
	}
	for _, h := range in.HoursUTC {
		c.num("hours_utc", "Hour", float64(h), 1, 24)
	}
	if hasDuplicate(in.HoursUTC) {
		c.add("hours_utc", "Hours: remove duplicates")
	}
	req.Hours = in.HoursUTC
	if len(in.FrequenciesMHz) == 0 || len(in.FrequenciesMHz) > maxHFFreqs {
		c.add(freqField, "Frequencies: between 1 and 30 values")
	}
	for _, f := range in.FrequenciesMHz {
		c.num(freqField, "Frequency", f, 1, 30)
	}
	if hasDuplicate(in.FrequenciesMHz) {
		c.add(freqField, "Frequencies: remove duplicates")
	}
	req.Frequencies = in.FrequenciesMHz

	noise, ok := hfNoise[in.NoiseEnvironment]
	if !ok {
		c.add("noise_environment", "Noise environment: unknown")
	}
	req.ManMadeNoise = noise

	req.Digital.RequiredSIR = 10
	switch in.Modulation {
	case "", "analog":
	case "digital":
		req.Modulation = hf.Digital
		if d := in.Digital; d != nil {
			req.Digital = hf.DigitalParams{
				RequiredSIR: c.num("digital.required_sir_db", "Required SIR", d.RequiredSIRDB, -30, 200),
				F0:          c.num("digital.f0_hz", "F0", d.F0Hz, 0, 1000),
				T0:          c.num("digital.t0_ms", "T0", d.T0Ms, 0, 1000),
				A:           c.num("digital.a_db", "A", d.ADB, 0, 1000),
				TW:          c.num("digital.tw_ms", "TW", d.TWMs, 0, 50),
				FW:          c.num("digital.fw_hz", "FW", d.FWHz, 0, 1000),
			}
		}
	default:
		c.add("modulation", "Modulation: must be analog or digital")
	}
	return req
}

// hasDuplicate reports whether a value occurs more than once in s.
func hasDuplicate[T comparable](s []T) bool {
	seen := make(map[T]bool, len(s))
	for _, v := range s {
		if seen[v] {
			return true
		}
		seen[v] = true
	}
	return false
}

// hfPattern returns the gain in dBi of a stored antenna at an azimuth offset from its main lobe and an elevation above
// the horizon: its gain less the attenuation of its patterns, as for the links.
func hfPattern(a store.Antenna) func(azimuthOffset, elevation float64) float64 {
	lobe := antenna.Lobe{H: a.PatternH, V: a.PatternV}
	return func(azimuthOffset, elevation float64) float64 {
		return a.GainDBi - lobe.AttenuationDB(azimuthOffset, elevation)
	}
}

// finite returns v, or the "does not apply" value for NaN and infinities, which JSON cannot carry.
func finite(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return hfNotAvailable
	}
	return v
}

// toHFPoint returns the API view of a point of the model.
func toHFPoint(p hf.Point) HFPoint {
	return HFPoint{
		Month: p.Month, HourUTC: p.Hour, FrequencyMHz: p.Frequency,
		DistanceKm: finite(p.Distance), MUFBasic: finite(p.BMUF), MUF10: finite(p.MUF10), MUF50: finite(p.MUF50),
		MUF90: finite(p.MUF90), MUFOper: finite(p.OPMUF),
		FieldStrengthDBuV: finite(p.FieldStrength), RxPowerDBW: finite(p.Pr), NoiseDB: finite(p.NoiseTotal),
		SNRDB: finite(p.SNR), SNRPercentileDB: finite(p.SNRXX),
		BasicReliabilityPct: finite(p.BCR), OverallReliabilityPct: finite(p.OCR),
		DominantMode: p.Dominant, DominantElevationDeg: finite(p.DomElevation), DominantDelayMs: finite(p.DomDelay),
		DominantLossDB: finite(p.DomLoss), DominantSupportedPct: finite(p.DomFprob),
		DominantReflectionKm: finite(p.DomHeight), DominantFieldStrength: finite(p.DomFieldStr),
	}
}

// registerHF registers the HF propagation prediction (ITU-R P.533) between two of the user's sites.
func (s *Server) registerHF(api huma.API) {
	o := op("predict-hf", http.MethodPost, "/api/hf/predictions", scopeHFRead,
		"HF circuit prediction between two sites (ITU-R P.533), nothing is stored")
	o.Errors = append(o.Errors, http.StatusUnprocessableEntity, http.StatusTooManyRequests, http.StatusServiceUnavailable)
	huma.Register(api, o, func(ctx context.Context, in *struct{ Body HFPredictionInput }) (*HFPredictionOutput, error) {
		if s.HF == nil {
			return nil, huma.Error503ServiceUnavailable("The HF model is not configured on this server")
		}
		owner := userOf(ctx).ID
		if err := s.computeHit(ctx, owner); err != nil {
			return nil, err
		}
		a, err := s.Store.GetSite(ctx, owner, in.Body.SiteAID)
		if err != nil {
			return nil, s.storeErr(err)
		}
		b, err := s.Store.GetSite(ctx, owner, in.Body.SiteBID)
		if err != nil {
			return nil, s.storeErr(err)
		}
		req, err := parseHF(in.Body, a, b)
		if err != nil {
			return nil, err
		}
		for _, e := range []struct {
			id  *int64
			dst *hf.Antenna
		}{{in.Body.TxAntennaID, &req.TxAntenna}, {in.Body.RxAntennaID, &req.RxAntenna}} {
			if e.id == nil {
				continue
			}
			ant, err := s.Store.GetAntenna(ctx, owner, *e.id)
			if err != nil {
				return nil, s.storeErr(err)
			}
			e.dst.Pattern = hfPattern(ant)
		}
		res, err := s.HF.Predict(req)
		if err != nil {
			s.Log.Error("hf prediction failed", "err", err)
			return nil, huma.Error500InternalServerError("Internal error")
		}
		out := make([]HFPoint, 0, len(res.Points))
		for _, p := range res.Points {
			out = append(out, toHFPoint(p))
		}
		return &HFPredictionOutput{HFPrediction{Points: out}}, nil
	})
}
