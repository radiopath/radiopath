package web

import (
	"fmt"
	"html/template"
	"log/slog"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/radiopath/radiopath/internal/antenna"
	"github.com/radiopath/radiopath/internal/link"
	"github.com/radiopath/radiopath/internal/plot"
	"github.com/radiopath/radiopath/internal/store"
)

func TestRenderPages(t *testing.T) {
	s := &Server{Log: slog.Default(), Registration: true, Version: "0.1.0-42-gdeadbee"}
	s.Handler()

	site := store.Site{ID: 1, Name: "Säntis", Lat: 47.24907, Lon: 9.34317, AntennaHeightM: 25}
	sites := []store.Site{site}
	res := &link.Result{ClutterLossDB: 40, HasCanopy: true, CanopyPathBM: 45, VegetationLossDB: 32.6}
	now := time.Now()

	antID := int64(1)
	pat := make([]float64, antenna.Steps)
	for i := range pat {
		pat[i] = 10 * (1 - math.Cos(float64(i)*math.Pi/180))
	}

	rx := -42.9
	measured := store.LinkWithSites{SiteA: site, SiteB: site}
	measured.ID, measured.Name = 1, "Säntis - Hoher Kasten"
	measured.MeasuredRxDBm, measured.ClutterLossDB = &rx, 40
	measured.ShareToken, measured.ShareExpires = "tok", &now

	detail := linkDetailData{Link: measured, Result: res, SVG: template.HTML(plot.SVG(*res, 10, 10)),
		TxAntenna: "Yagi 9el", TxAzimuthDeg: 42, Reliability: link.ReliabilityPct,
		MeasuredDBm: rx, MeasuredDeltaDB: -2.4,
		Path: "/links/1", ShareURL: "https://example.org/s/l/tok"}

	shared := detail
	shared.Shared, shared.Path, shared.ShareURL = true, "/s/l/tok", ""

	cov := store.CoverageWithSite{Site: site}
	cov.ID, cov.Name, cov.FreqMHz, cov.RangeM, cov.ResolutionM = 1, "Alpstein", 145.5, 30000, 100
	cov.ComputedAt, cov.ComputeMs = &now, 1234
	cov.Bounds = &store.Bounds{North: 48, South: 46, East: 10, West: 8}
	cov.ShareToken, cov.ShareExpires = "tok", &now
	cov.ViewOpacity, cov.ViewOverlays, cov.ViewCircles = 40, []int64{1}, true
	overlays := []overlay{{CoverageWithSite: cov, URL: "/coverages/1/image.png?t=1", Checked: true}}
	sharedOverlays := []overlay{{CoverageWithSite: cov, URL: "/s/c/tok/image.png?id=1&t=1", Checked: true}}

	cases := []struct {
		page string
		data any
	}{
		{"login", loginData{Next: "/", Registration: true, Forgot: true, Notice: notices["registered"]}},
		{"account", accountData{base: base{User: "hb9hil"}, Email: "op@example.org", Verified: true, Notice: notices["sent"]}},
		{"account", accountData{base: base{User: "hb9hil"}, Errors: []string{"Current password is wrong"}}},
		{"forgot", forgotData{Notice: notices["forgot"]}},
		{"notfound", base{}},
		{"reset", resetData{Token: "abc"}},
		{"reset", resetData{Errors: []string{staleLink}}},
		{"login", loginData{Maintenance: true}},
		{"login", loginData{Notice: notices["unverified"], Unverified: "hb9hil", Forgot: true}},
		{"admin_login", adminLoginData{base: base{Admin: true}, Error: "Wrong token."}},
		{"admin_users", adminData{base: base{Admin: true}, Notice: "hb9hil: 2 session(s) revoked",
			Users:       []store.UserStat{{User: store.User{ID: 1, Name: "hb9hil", CreatedAt: now}, Sessions: 2, LastLogin: &now}},
			PendingUser: "hb9hil", Maintenance: true}},
		{"admin_users", adminData{base: base{Admin: true}, Errors: []string{"No such user: nope"}}},
		{"register", registerData{Name: "hb9hil", Email: "op@example.org", Errors: []string{"nope"}}},
		{"sites_list", struct {
			base
			Sites []store.Site
			Error string
		}{Sites: sites}},
		{"sites_list", struct {
			base
			Sites []store.Site
			Error string
		}{}},
		{"site_form", siteFormData{Site: site, Form: siteValues(site)}},
		{"antennas_list", antennasData{Antennas: antennaViews([]store.Antenna{
			{ID: 1, Name: "Sector 90", GainDBi: 16.3, FreqMHz: 145, PatternH: pat, Source: "sector90.msi"},
		})}},
		{"antennas_list", antennasData{Errors: []string{"antenna: no GAIN in the file"}}},
		{"antennas_list", antennasData{Antennas: antennaViews([]store.Antenna{
			{ID: 2, Name: "Sector 65", GainDBi: 15, PatternH: antenna.Sector(65, 25), Source: "manual"},
		}), Errors: []string{"Gain: not a number"}, Edit: 2}},
		{"link_form", linkFormData{Link: store.Link{TxAntennaID: &antID}, Sites: sites,
			Antennas: []store.Antenna{{ID: 1, Name: "Sector 90"}}, Form: linkValues(store.Link{})}},
		{"coverage_form", coverageFormData{Coverage: cov.Coverage, Sites: sites,
			Antennas: []store.Antenna{{ID: 1, Name: "Sector 90"}}, Form: coverageValues(cov.Coverage)}},
		{"links_list", struct {
			base
			Links []store.LinkWithSites
			Sites int
		}{}},
		{"links_list", struct {
			base
			Links []store.LinkWithSites
			Sites int
		}{Sites: 1}},
		{"coverages_list", struct {
			base
			Coverages []store.CoverageWithSite
			Sites     int
		}{Coverages: []store.CoverageWithSite{cov}, Sites: 1}},
		{"link_detail", detail},
		{"link_detail", shared},
		{"coverage_detail", coverageDetailData{Coverage: cov, legendData: legendViews(plot.DefaultLegend()), Others: overlays,
			TxAntenna: "Sector 90", Path: "/coverages/1", ShareURL: "https://example.org/s/c/tok",
			ViewURL: "/coverages/1/view"}},
		{"coverage_detail", coverageDetailData{Coverage: cov, legendData: legendViews(plot.DefaultLegend()), Others: sharedOverlays,
			TxAntenna: "Sector 90", Path: "/s/c/tok", Shared: true}},
	}

	outDir := os.Getenv("RADIOPATH_TEST_OUT")

	for i, c := range cases {
		var b strings.Builder
		if err := s.tmpl[c.page].Execute(&b, c.data); err != nil {
			t.Fatalf("%s: %v", c.page, err)
		}
		out := b.String()
		if outDir != "" {
			name := fmt.Sprintf("%s/%02d_%s.html", outDir, i, c.page)
			if err := os.WriteFile(name, []byte(out), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if strings.Contains(out, "<no value>") {
			t.Errorf("%s: rendered a missing value", c.page)
		}
		for _, want := range []string{`href="/static/app.css`, "cp-theme", "</html>"} {
			if !strings.Contains(out, want) {
				t.Errorf("%s: missing %q", c.page, want)
			}
		}
	}
}
