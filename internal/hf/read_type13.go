package hf

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const (
	antAzimuths   = 360 // fixed number of azimuths at 1-degree intervals
	antElevations = 91  // fixed number of elevations at 1-degree intervals
)

// scanNumbers mimics sscanf with a sequence of %d/%lf conversions: it parses
// consecutive numbers at the start of line, stopping at the first thing that
// is not a number. Fixed width columns such as "-99.999-15.670" are split the
// way strtod splits them.
func scanNumbers(line string) []float64 {
	var out []float64
	i := 0
	for {
		for i < len(line) && (line[i] == ' ' || line[i] == '\t' || line[i] == '\r' || line[i] == '\n') {
			i++
		}
		start := i
		if i < len(line) && (line[i] == '+' || line[i] == '-') {
			i++
		}
		digits := 0
		for i < len(line) && line[i] >= '0' && line[i] <= '9' {
			i++
			digits++
		}
		if i < len(line) && line[i] == '.' {
			i++
			for i < len(line) && line[i] >= '0' && line[i] <= '9' {
				i++
				digits++
			}
		}
		if digits == 0 {
			return out
		}
		if i < len(line) && (line[i] == 'e' || line[i] == 'E') {
			j := i + 1
			if j < len(line) && (line[j] == '+' || line[j] == '-') {
				j++
			}
			if j < len(line) && line[j] >= '0' && line[j] <= '9' {
				for j < len(line) && line[j] >= '0' && line[j] <= '9' {
					j++
				}
				i = j
			}
		}
		v, err := strconv.ParseFloat(line[start:i], 64)
		if err != nil {
			return out
		}
		out = append(out, v)
	}
}

// lineReader reads text lines like fgets; at the end of the file it returns
// empty lines.
type lineReader struct {
	sc *bufio.Scanner
}

// newLineReader wraps r in a lineReader.
func newLineReader(r io.Reader) *lineReader {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	return &lineReader{sc: sc}
}

// next returns the next line without its line terminator.
func (l *lineReader) next() string {
	if !l.sc.Scan() {
		return ""
	}
	return strings.TrimRight(l.sc.Text(), "\r\n")
}

// allocateAntennaMemory creates a zeroed pattern for freqn frequencies.
func allocateAntennaMemory(ant *antenna, freqn, azin, elen int) {
	ant.freqn = freqn
	ant.freqs = make([]float64, freqn)
	ant.pattern = make([][][]float64, freqn)
	for i := range ant.pattern {
		ant.pattern[i] = make([][]float64, azin)
		for j := range ant.pattern[i] {
			ant.pattern[i][j] = make([]float64, elen)
		}
	}
}

// isotropicPattern fills the pattern with the constant gain g (dBi).
func isotropicPattern(ant *antenna, g float64) {
	allocateAntennaMemory(ant, 1, antAzimuths, antElevations)
	ant.freqs[0] = 0
	for i := 0; i < antAzimuths; i++ {
		for j := 0; j < antElevations; j++ {
			ant.pattern[0][i][j] = g
		}
	}
	ant.name = "ISOTROPIC"
}

// assign stores the parsed values into dst, as many as were parsed (sscanf
// leaves the remaining targets untouched).
func assign(dst []float64, vals []float64) {
	copy(dst, vals)
}

// readElevationBlock reads the 91 elevation values of one azimuth (or
// frequency) block: nine lines of ten values and a last line with one value.
// first holds the values of an already parsed first line (type 13 and 14).
func readElevationBlock(lr *lineReader, dst []float64, first []float64) {
	j := 0
	if first != nil {
		assign(dst[0:10], first)
		j = 10
	}
	for ; j < 90; j += 10 {
		assign(dst[j:j+10], scanNumbers(lr.next()))
	}
	assign(dst[90:91], scanNumbers(lr.next()))
}

// readType11 reads a VOACAP type 11 antenna file: gain versus elevation, the
// same for all azimuths.
func readType11(ant *antenna, lr *lineReader) {
	allocateAntennaMemory(ant, 1, antAzimuths, antElevations)

	ant.name = lr.next()

	lr.next() // number of parameters

	// Max gain
	maxG := 0.0
	if v := scanNumbers(lr.next()); len(v) > 0 {
		maxG = v[0]
	}
	lr.next() // antenna type (ignored)
	lr.next() // efficiency (ignored)

	ant.freqs[0] = 0

	readElevationBlock(lr, ant.pattern[0][0], nil)

	// If max gain != 0.0 add it to the values read in from the table.
	if maxG != 0.0 {
		for j := 0; j < antElevations; j++ {
			ant.pattern[0][0][j] += maxG
		}
	}

	// Copy the elevation data to the other azimuths.
	for j := 1; j < antAzimuths; j++ {
		copy(ant.pattern[0][j], ant.pattern[0][0])
	}
}

// readType13 reads a VOACAP type 13 antenna file (360 x 91 gain table) and
// rotates the pattern so that the main beam points to bearing (radians), to
// the nearest degree.
func readType13(ant *antenna, lr *lineReader, bearing float64) {
	allocateAntennaMemory(ant, 1, antAzimuths, antElevations)

	// The azimuth offset of the main beam, truncated to an integer degree.
	iMBOS := int(bearing * r2d)

	ant.name = lr.next()

	lr.next() // number of parameters
	lr.next() // max gain (not added for type 13)
	lr.next() // antenna type
	lr.next() // efficiency
	if v := scanNumbers(lr.next()); len(v) > 0 {
		ant.freqs[0] = v[0] // atof of the frequency line
	}

	for i := 0; i < antAzimuths; i++ {
		// Advance to the next azimuth and roll it over if necessary.
		iazi := (iMBOS + i) % antAzimuths

		// The first line starts with the azimuth label.
		v := scanNumbers(lr.next())
		var first []float64
		if len(v) > 1 {
			first = v[1:]
		} else {
			first = []float64{}
		}
		readElevationBlock(lr, ant.pattern[0][iazi], first)
	}
}

// readType14 reads a VOACAP type 14 antenna file: 30 frequencies of 91
// elevation gains, the same for all azimuths.
func readType14(ant *antenna, lr *lineReader) {
	allocateAntennaMemory(ant, 30, antAzimuths, antElevations)

	ant.name = lr.next()

	lr.next() // number of parameters

	maxG := 0.0
	if v := scanNumbers(lr.next()); len(v) > 0 {
		maxG = v[0]
	}
	lr.next() // antenna type
	lr.next() // frequency

	for i := 0; i < 30; i++ {
		// The first line holds the frequency, the efficiency and ten gains.
		v := scanNumbers(lr.next())
		var first []float64
		if len(v) > 0 {
			ant.freqs[i] = v[0]
		}
		if len(v) > 2 {
			first = v[2:]
		} else {
			first = []float64{}
		}
		readElevationBlock(lr, ant.pattern[i][0], first)

		if maxG != 0.0 {
			for j := 0; j <= 90; j++ {
				ant.pattern[i][0][j] += maxG
			}
		}

		for j := 1; j < antAzimuths; j++ {
			copy(ant.pattern[i][j], ant.pattern[i][0])
		}
	}
}

// readAntennaFile reads a VOACAP type 11, 13 or 14 antenna file. The type is
// the first number on the fourth line. bearing is the direction of the main
// beam for type 13.
func readAntennaFile(name string, bearing float64) (*antenna, error) {
	fp, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOpenAntennaFile, err)
	}
	defer fp.Close()

	return parseAntenna(fp, bearing)
}

// parseAntenna reads a VOACAP antenna file from r, see readAntennaFile.
func parseAntenna(r io.Reader, bearing float64) (*antenna, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOpenAntennaFile, err)
	}

	// Scroll to the antenna type on line four.
	head := newLineReader(strings.NewReader(string(data)))
	line := ""
	for i := 0; i < 4; i++ {
		line = head.next()
	}
	antType := -1
	if v := scanNumbers(line); len(v) > 0 {
		antType = int(v[0])
	}

	lr := newLineReader(strings.NewReader(string(data)))
	ant := new(antenna)
	switch antType {
	case 11:
		readType11(ant, lr)
	case 13:
		readType13(ant, lr, bearing)
	case 14:
		readType14(ant, lr)
	default:
		return nil, fmt.Errorf("%w: %d", ErrAntennaType, antType)
	}
	return ant, nil
}
