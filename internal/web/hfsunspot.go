package web

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// HFSunspot is the smoothed sunspot number of a month, the value for the prediction's ssn field.
type HFSunspot struct {
	SSN       int    `json:"ssn" doc:"12-month smoothed sunspot number (R12), 1 to 311"`
	Source    string `json:"source" enum:"observed,predicted" doc:"observed: smoothed from measurements; predicted: forecast of NOAA SWPC"`
	Reference string `json:"reference" doc:"where the value comes from"`
}

// HFSunspotInput selects the month.
type HFSunspotInput struct {
	Year  int `query:"year" minimum:"1749" maximum:"2100" doc:"Year"`
	Month int `query:"month" minimum:"1" maximum:"12" doc:"Month, 1 to 12"`
}

// HFSunspotOutput wraps HFSunspot.
type HFSunspotOutput struct{ Body HFSunspot }

// registerHFSunspot registers the lookup of the sunspot number of a month, loaded from NOAA SWPC.
func (s *Server) registerHFSunspot(api huma.API) {
	o := op("hf-sunspot", http.MethodGet, "/api/hf/sunspots", scopeHFRead,
		"Smoothed sunspot number of a month (NOAA SWPC), 404 when the server has none")
	o.Errors = append(o.Errors, http.StatusNotFound)
	huma.Register(api, o, func(_ context.Context, in *HFSunspotInput) (*HFSunspotOutput, error) {
		if s.Sunspots == nil {
			return nil, huma.Error404NotFound("No sunspot data on this server")
		}
		v, ok := s.Sunspots.Lookup(in.Year, in.Month)
		if !ok {
			return nil, huma.Error404NotFound("No sunspot number for this month")
		}
		out := &HFSunspotOutput{Body: HFSunspot{SSN: v.SSN, Source: "observed", Reference: "NOAA SWPC"}}
		if v.Predicted {
			out.Body.Source = "predicted"
		}
		return out, nil
	})
}
