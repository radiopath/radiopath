package hf

import (
	"bufio"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// testDirs returns the directory with the ITU data files (see
// scripts/fetch-hf-data.sh) and the one with the reference vectors of the C
// code, or skips the test when the data is missing.
func testDirs(t *testing.T) (dataDir, refDir string) {
	t.Helper()
	refDir = "testdata"
	dataDir = filepath.Join("..", "..", "tmp", "testfiles", "hf1")
	if root := os.Getenv("RADIOPATH_DATASET_DIR"); root != "" {
		dataDir = filepath.Join(root, "hf1")
	}
	if _, err := os.Stat(filepath.Join(dataDir, "ionos01.bin")); err != nil {
		if os.Getenv("HF_REQUIRE_DATA") != "" {
			t.Fatalf("P.533 data files not found in %s", dataDir)
		}
		t.Skipf("P.533 data files not found in %s (run scripts/fetch-hf-data.sh or set RADIOPATH_DATASET_DIR)", dataDir)
	}
	return dataDir, refDir
}

// parseList parses a comma separated list of numbers.
func parseList(s string) []float64 {
	var out []float64
	for _, f := range strings.Split(s, ",") {
		v, err := strconv.ParseFloat(strings.TrimSpace(f), 64)
		if err != nil {
			break
		}
		out = append(out, v)
	}
	return out
}

// quoted returns the text between the first and the last double quote.
func quoted(s string) string {
	i := strings.Index(s, "\"")
	j := strings.LastIndex(s, "\"")
	if i < 0 || j <= i {
		return ""
	}
	return s[i+1 : j]
}

// localAntenna maps the antenna path of a reference input, written on the
// machine that ran the C code, to the file of the same name next to it.
func localAntenna(input, path string) string {
	base := path[strings.LastIndexAny(path, `\/`)+1:]
	return filepath.Join(filepath.Dir(input), base)
}

// readCaseInput reads a reference input file of the C driver into a Request.
func readCaseInput(t *testing.T, name string) Request {
	t.Helper()
	fp, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer fp.Close()

	req := Request{ManMadeNoise: NoiseNoisy}
	sc := bufio.NewScanner(fp)
	for sc.Scan() {
		line := sc.Text()
		key, rest, _ := strings.Cut(line, " ")
		rest = strings.TrimSpace(rest)
		num := func() float64 {
			f := strings.Fields(rest)
			if len(f) == 0 {
				return 0
			}
			v, _ := strconv.ParseFloat(f[0], 64)
			return v
		}
		switch key {
		case "Path.L_tx.lat":
			req.TxLat = num()
		case "Path.L_tx.lng":
			req.TxLng = num()
		case "Path.L_rx.lat":
			req.RxLat = num()
		case "Path.L_rx.lng":
			req.RxLng = num()
		case "TXAntFilePath":
			if q := quoted(rest); q != "ISOTROPIC" {
				req.TxAntenna.File = localAntenna(name, q)
			}
		case "RXAntFilePath":
			if q := quoted(rest); q != "ISOTROPIC" {
				req.RxAntenna.File = localAntenna(name, q)
			}
		case "TXBearing":
			b := num()
			req.TxAntenna.Bearing = &b
		case "RXBearing":
			b := num()
			req.RxAntenna.Bearing = &b
		case "TXGOS":
			req.TxAntenna.GainOffset = num()
		case "RXGOS":
			req.RxAntenna.GainOffset = num()
		case "Path.year":
			req.Year = int(num())
		case "Path.month":
			req.Month = int(num())
		case "Path.hour":
			for _, v := range parseList(rest) {
				req.Hours = append(req.Hours, int(v))
			}
		case "Path.SSN":
			req.SSN = int(num())
		case "Path.frequency":
			req.Frequencies = parseList(rest)
		case "Path.txpower":
			req.TxPower = num()
		case "Path.BW":
			req.Bandwidth = num()
		case "Path.SNRr":
			req.RequiredSNR = num()
		case "Path.SNRXXp":
			req.SNRXXPercent = int(num())
		case "Path.ManMadeNoise":
			if v, ok := ParseNoiseEnvironment(quoted(rest)); ok {
				req.ManMadeNoise = v
			} else {
				req.ManMadeNoise = num()
			}
		case "Path.Modulation":
			if quoted(rest) == "DIGITAL" {
				req.Modulation = Digital
			}
		case "Path.SIRr":
			req.Digital.RequiredSIR = num()
		case "Path.A":
			req.Digital.A = num()
		case "Path.TW":
			req.Digital.TW = num()
		case "Path.FW":
			req.Digital.FW = num()
		case "Path.T0":
			req.Digital.T0 = num()
		case "Path.F0":
			req.Digital.F0 = num()
		case "Path.SorL":
			req.LongPath = quoted(rest) == "LONGPATH"
		case "LL.lat":
			req.RxLat = num()
		case "LL.lng":
			req.RxLng = num()
		}
	}
	return req
}

// refRow is a data row of a reference CSV report.
type refRow struct {
	fields []string
}

// readRefCSV reads the data rows (starting "MM, HH,") of a report.
func readRefCSV(t *testing.T, name string) []refRow {
	t.Helper()
	fp, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer fp.Close()

	var rows []refRow
	sc := bufio.NewScanner(fp)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if len(line) < 6 || line[2] != ',' || line[3] != ' ' || line[0] < '0' || line[0] > '9' {
			continue
		}
		f := strings.Split(line, ",")
		for i := range f {
			f[i] = strings.TrimSpace(f[i])
		}
		rows = append(rows, refRow{fields: f})
	}
	return rows
}

// column pairs a report column with the value of a point.
type column struct {
	name string
	get  func(p *Point) float64
}

// reportColumns lists the numeric columns of the report starting at column 4
// (1-based), in the order of the C driver. nil entries are strings.
func reportColumns() []column {
	c := func(n string, g func(p *Point) float64) column { return column{n, g} }
	return []column{
		c("RxLat", nil), c("RxLng", nil),
		c("distance", func(p *Point) float64 { return p.Distance }),
		c("dmax", func(p *Point) float64 { return p.Dmax }),
		c("slant", func(p *Point) float64 { return p.SlantRange }),
		c("elevation", func(p *Point) float64 { return p.Elevation }),
		c("BMUF", func(p *Point) float64 { return p.BMUF }),
		c("MUF50", func(p *Point) float64 { return p.MUF50 }),
		c("MUF90", func(p *Point) float64 { return p.MUF90 }),
		c("MUF10", func(p *Point) float64 { return p.MUF10 }),
		c("OPMUF", func(p *Point) float64 { return p.OPMUF }),
		c("OPMUF90", func(p *Point) float64 { return p.OPMUF90 }),
		c("OPMUF10", func(p *Point) float64 { return p.OPMUF10 }),
		c("n0F2", nil), c("n0E", nil),
		c("Ep", func(p *Point) float64 { return p.FieldStrength }),
		c("Pr", func(p *Point) float64 { return p.Pr }),
		c("Grw", func(p *Point) float64 { return p.RxGain }),
		c("FaA", func(p *Point) float64 { return p.NoiseAtmospheric }),
		c("FaM", func(p *Point) float64 { return p.NoiseManMade }),
		c("FaG", func(p *Point) float64 { return p.NoiseGalactic }),
		c("DuA", func(p *Point) float64 { return p.DuA }),
		c("DlA", func(p *Point) float64 { return p.DlA }),
		c("DuM", func(p *Point) float64 { return p.DuM }),
		c("DlM", func(p *Point) float64 { return p.DlM }),
		c("DuG", func(p *Point) float64 { return p.DuG }),
		c("DlG", func(p *Point) float64 { return p.DlG }),
		c("DuT", func(p *Point) float64 { return p.DuT }),
		c("DlT", func(p *Point) float64 { return p.DlT }),
		c("FamT", func(p *Point) float64 { return p.NoiseTotal }),
		c("SNR", func(p *Point) float64 { return p.SNR }),
		c("DuSN", func(p *Point) float64 { return p.DuSN }),
		c("DlSN", func(p *Point) float64 { return p.DlSN }),
		c("SNRXX", func(p *Point) float64 { return p.SNRXX }),
		c("SIR", func(p *Point) float64 { return p.SIR }),
		c("DuSI", func(p *Point) float64 { return p.DuSI }),
		c("DlSI", func(p *Point) float64 { return p.DlSI }),
		c("RSN", func(p *Point) float64 { return p.RSN }),
		c("RT", func(p *Point) float64 { return p.RT }),
		c("RF", func(p *Point) float64 { return p.RF }),
		c("BCR", func(p *Point) float64 { return p.BCR }),
		c("OCR", func(p *Point) float64 { return p.OCR }),
		c("OCRs", func(p *Point) float64 { return p.OCRs }),
		c("probocc", func(p *Point) float64 { return p.ProbOcc }),
		c("MIR", func(p *Point) float64 { return p.MIR }),
		c("dominant", nil),
		c("domElevation", func(p *Point) float64 { return p.DomElevation }),
		c("domDelay", func(p *Point) float64 { return p.DomDelay }),
		c("domLoss", func(p *Point) float64 { return p.DomLoss }),
		c("domFprob", func(p *Point) float64 { return p.DomFprob }),
		c("domHeight", func(p *Point) float64 { return p.DomHeight }),
		c("domPower", func(p *Point) float64 { return p.DomPower }),
		c("domRxGain", func(p *Point) float64 { return p.DomRxGain }),
		c("domFieldStr", func(p *Point) float64 { return p.DomFieldStr }),
		c("domBMUF", func(p *Point) float64 { return p.DomBMUF }),
		c("Es", func(p *Point) float64 { return p.Es }),
		c("El", func(p *Point) float64 { return p.El }),
		c("E0", func(p *Point) float64 { return p.E0 }),
		c("Gap", func(p *Point) float64 { return p.Gap }),
		c("Ly", func(p *Point) float64 { return p.Ly }),
		c("fM", func(p *Point) float64 { return p.FM }),
		c("fL", func(p *Point) float64 { return p.FL }),
		c("K0", func(p *Point) float64 { return p.K[0] }),
		c("K1", func(p *Point) float64 { return p.K[1] }),
		c("Gtl", func(p *Point) float64 { return p.Gtl }),
		c("Grw08", func(p *Point) float64 { return p.RxGain }),
		c("fH", func(p *Point) float64 { return p.FH }),
		c("F", func(p *Point) float64 { return p.F }),
	}
}

// decimals returns the number of decimals of a printed number.
func decimals(s string) int {
	i := strings.IndexByte(s, '.')
	if i < 0 {
		return 0
	}
	return len(s) - i - 1
}

// modeName formats a lowest-order mode column like the report does.
func modeName(n int, layer string) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%d%s", n, layer)
}

// compareRow compares a point to a report row field by field. The report
// prints rounded values, so the tolerance is half a unit of the last printed
// digit (plus rounding noise).
func compareRow(p *Point, row refRow) []string {
	var diffs []string
	f := row.fields
	if len(f) < 71 {
		return []string{fmt.Sprintf("only %d fields", len(f))}
	}
	cols := reportColumns()
	for i, col := range cols {
		s := f[3+i]
		if col.get == nil {
			continue
		}
		want, err := strconv.ParseFloat(s, 64)
		if err != nil {
			diffs = append(diffs, fmt.Sprintf("%s: bad reference %q", col.name, s))
			continue
		}
		got := col.get(p)
		// The report rounds to the printed digits: equal strings mean that the
		// values agree to far better than the last printed digit.
		if strconv.FormatFloat(got, 'f', decimals(s), 64) != s {
			diffs = append(diffs, fmt.Sprintf("%s: got %.9f want %s (%.3g)", col.name, got, s, got-want))
		}
	}
	// The mode columns
	n0F2 := strings.TrimSuffix(f[3+13], "")
	if want := modeName(p.LowestF2Hop, "F2"); n0F2 != want && !(want == "" && (n0F2 == "NONE" || n0F2 == "")) {
		diffs = append(diffs, fmt.Sprintf("n0F2: got %q want %q", want, n0F2))
	}
	n0E := f[3+14]
	if want := modeName(p.LowestEHop, "E"); n0E != want && !(want == "" && (n0E == "NONE" || n0E == "")) {
		diffs = append(diffs, fmt.Sprintf("n0E: got %q want %q", want, n0E))
	}
	if f[3+45] != p.Dominant {
		diffs = append(diffs, fmt.Sprintf("dominant: got %q want %q", p.Dominant, f[3+45]))
	}
	return diffs
}

// TestReferenceCases compares the prediction of every reference case with the
// report written by the C implementation.
func TestReferenceCases(t *testing.T) {
	dataDir, refDir := testDirs(t)
	d, err := Open(dataDir, "")
	if err != nil {
		t.Fatal(err)
	}
	inputs, _ := filepath.Glob(filepath.Join(refDir, "[cx]*??.in"))
	if len(inputs) == 0 {
		t.Fatal("no reference cases in testdata")
	}
	for _, in := range inputs {
		name := strings.TrimSuffix(filepath.Base(in), ".in")
		t.Run(name, func(t *testing.T) {
			req := readCaseInput(t, in)
			rows := readRefCSV(t, strings.TrimSuffix(in, ".in")+".csv")
			res, err := d.Predict(req)
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Points) != len(rows) {
				t.Fatalf("got %d points, reference has %d rows", len(res.Points), len(rows))
			}
			bad := 0
			for i := range res.Points {
				diffs := compareRow(&res.Points[i], rows[i])
				if len(diffs) > 0 {
					bad++
					if bad <= 5 {
						t.Errorf("hour %d f %.3f: %s", res.Points[i].Hour, res.Points[i].Frequency, strings.Join(diffs, "; "))
					}
				}
			}
			if bad > 0 {
				t.Errorf("%d of %d rows differ", bad, len(rows))
			}
		})
	}
}

// TestDecileFactors checks the decile factors read from the P.1239-3 file:
// lower deciles are below one and upper deciles above.
func TestDecileFactors(t *testing.T) {
	dataDir, _ := testDirs(t)
	d, err := Open(dataDir, "")
	if err != nil {
		t.Fatal(err)
	}
	for s := range d.foF2var {
		for h := range d.foF2var[s] {
			for lat := range d.foF2var[s][h] {
				for ssn := range d.foF2var[s][h][lat] {
					lo := d.foF2var[s][h][lat][ssn][dl]
					up := d.foF2var[s][h][lat][ssn][du]
					if !(0.3 < lo && lo < 1.0 && 1.0 < up && up < 3.0) {
						t.Fatalf("season %d hour %d lat %d ssn %d: lower %v upper %v", s, h, lat, ssn, lo, up)
					}
				}
			}
		}
	}
}

// TestPredictConcurrent runs predictions of several requests in parallel and
// checks that they give the same result as one after the other.
func TestPredictConcurrent(t *testing.T) {
	dataDir, refDir := testDirs(t)
	d, err := Open(dataDir, "")
	if err != nil {
		t.Fatal(err)
	}
	var reqs []Request
	for _, n := range []string{"case01", "case03", "case07", "case12"} {
		reqs = append(reqs, readCaseInput(t, filepath.Join(refDir, n+".in")))
	}
	want := make([]*Result, len(reqs))
	for i, r := range reqs {
		if want[i], err = d.Predict(r); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	for rep := 0; rep < 4; rep++ {
		for i, r := range reqs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				got, err := d.Predict(r)
				if err != nil {
					t.Error(err)
					return
				}
				if len(got.Points) != len(want[i].Points) {
					t.Errorf("request %d: %d points, want %d", i, len(got.Points), len(want[i].Points))
					return
				}
				for k := range got.Points {
					if fmt.Sprintf("%+v", got.Points[k]) != fmt.Sprintf("%+v", want[i].Points[k]) {
						t.Errorf("request %d point %d differs", i, k)
						return
					}
				}
			}()
		}
	}
	wg.Wait()
}

// TestOpenMissingData checks the error of a directory without data files.
func TestOpenMissingData(t *testing.T) {
	if _, err := Open(t.TempDir(), ""); !errors.Is(err, ErrOpenP1239File) {
		t.Errorf("got %v, want ErrOpenP1239File", err)
	}
}

// TestReady checks that Ready names a missing data file and passes when all are present.
func TestReady(t *testing.T) {
	dir := t.TempDir()
	d := &Data{p533Dir: dir, p372Dir: dir}
	if err := d.Ready(); err == nil {
		t.Fatal("empty directory is ready")
	}
	names := []string{"P1239-3 Decile Factors.txt"}
	for m := 1; m <= 12; m++ {
		names = append(names, fmt.Sprintf("ionos%02d.bin", m), fmt.Sprintf("COEFF%02dW.txt", m))
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.Ready(); err != nil {
		t.Fatalf("complete directory not ready: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, "ionos07.bin")); err != nil {
		t.Fatal(err)
	}
	if err := d.Ready(); err == nil || !strings.Contains(err.Error(), "ionos07.bin") {
		t.Errorf("got %v, want an error naming ionos07.bin", err)
	}
}

// TestFunctionPattern checks the rotation of a pattern given as a function: the offset 0 lies at the bearing.
func TestFunctionPattern(t *testing.T) {
	gain := func(off, el float64) float64 { return off + el/100 }
	ant, err := functionPattern(gain, 10.5*d2r)
	if err != nil {
		t.Fatal(err)
	}
	if got := ant.pattern[0][10][0]; got != 0 {
		t.Errorf("main beam: got %v, want 0", got)
	}
	if got := ant.pattern[0][5][90]; got != 355.9 {
		t.Errorf("offset 355 at elevation 90: got %v, want 355.9", got)
	}
	if _, err := functionPattern(func(_, _ float64) float64 { return math.NaN() }, 0); err == nil {
		t.Error("NaN gain accepted")
	}
}

// TestPatternAntennaMatchesIsotropic checks that a constant gain function gives the prediction of an isotropic
// antenna with that gain, and that a directional one changes it.
func TestPatternAntennaMatchesIsotropic(t *testing.T) {
	dataDir, refDir := testDirs(t)
	d, err := Open(dataDir, "")
	if err != nil {
		t.Fatal(err)
	}
	req := readCaseInput(t, filepath.Join(refDir, "case01.in"))
	iso := req
	iso.TxAntenna.GainOffset = 3
	want, err := d.Predict(iso)
	if err != nil {
		t.Fatal(err)
	}
	fn := req
	fn.TxAntenna.Pattern = func(_, _ float64) float64 { return 3 }
	got, err := d.Predict(fn)
	if err != nil {
		t.Fatal(err)
	}
	for i := range want.Points {
		if got.Points[i].FieldStrength != want.Points[i].FieldStrength || got.Points[i].SNR != want.Points[i].SNR {
			t.Fatalf("point %d differs from the isotropic antenna", i)
		}
	}

	away := req
	away.TxAntenna.Pattern = func(off, _ float64) float64 {
		if off == 0 {
			return 3
		}
		return -30
	}
	back := req
	b := 180.0
	back.TxAntenna.Bearing = &b
	back.TxAntenna.Pattern = away.TxAntenna.Pattern
	aimed, _ := d.Predict(away)
	opposite, _ := d.Predict(back)
	if aimed.Points[0].FieldStrength <= opposite.Points[0].FieldStrength {
		t.Errorf("aimed %.1f dB not above the wrong way %.1f dB", aimed.Points[0].FieldStrength, opposite.Points[0].FieldStrength)
	}
}

// TestMonthRetry checks that a month whose files were missing loads once they are back.
func TestMonthRetry(t *testing.T) {
	src, _ := testDirs(t)
	src, err := filepath.Abs(src)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	link := func(name string) {
		if err := os.Symlink(filepath.Join(src, name), filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	link("P1239-3 Decile Factors.txt")
	d, err := Open(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.month(3); err == nil {
		t.Fatal("month without its files loaded")
	}
	link("ionos04.bin")
	link("COEFF04W.txt")
	if _, err := d.month(3); err != nil {
		t.Fatalf("month after the files are back: %v", err)
	}
}
