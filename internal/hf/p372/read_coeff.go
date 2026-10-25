package p372

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Lines of the COEFFxxW.txt files before the fakp block: the header line plus
// the ionospheric arrays (if2, ifm3, ie, iesu, ies, iels, ihpo1, ihpo2, ihp)
// that the noise code does not use.
const skippedLines = 1 + 400 + 181 + 84 + 114 + 175 + 114 + 155 + 202 + 138

// LoadCoefficients reads the atmospheric noise coefficients of one month from
// dir/COEFFxxW.txt (ReadFamDud() in the C code). month is 0-based, 0..11, as
// in path->month of P533; the file number is month+1. The binary COEFFxxW.BIN
// files are not used by the original code and are ignored.
func LoadCoefficients(dir string, month int) (*Coefficients, error) {
	if month < 0 || month > 11 {
		return nil, ErrMonth
	}

	name := filepath.Join(dir, fmt.Sprintf("COEFF%02dW.txt", month+1))
	f, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOpenCoeffFile, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for n := 0; n < skippedLines; n++ {
		if !sc.Scan() {
			return nil, fmt.Errorf("%w: %s ends before the noise data", ErrBadCoeffFile, name)
		}
	}

	c := new(Coefficients)

	// Each block is stored in Fortran order, so the flat index of the C
	// reshaping loops is the row-major index of the Go arrays.
	a, err := readBlock(sc, "fakp(29,16,6)", 29*16*6)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	for i := 0; i < 6; i++ {
		for j := 0; j < 16; j++ {
			for k := 0; k < 29; k++ {
				c.Fakp[i][j][k] = a[16*29*i+29*j+k]
			}
		}
	}

	a, err = readBlock(sc, "fakabp(2,6)", 2*6)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	for j := 0; j < 6; j++ {
		for k := 0; k < 2; k++ {
			c.Fakabp[j][k] = a[2*j+k]
		}
	}

	a, err = readBlock(sc, "dud(5,12,5)", 5*12*5)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	for i := 0; i < 5; i++ {
		for j := 0; j < 12; j++ {
			for k := 0; k < 5; k++ {
				c.Dud[i][j][k] = a[5*12*i+5*j+k]
			}
		}
	}

	a, err = readBlock(sc, "fam(14,12)", 12*14)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	for j := 0; j < 12; j++ {
		for k := 0; k < 14; k++ {
			c.Fam[j][k] = a[14*j+k]
		}
	}

	return c, nil
}

// readBlock reads the header line and then lines of numbers until count
// values have been collected.
func readBlock(sc *bufio.Scanner, header string, count int) ([]float64, error) {
	if !sc.Scan() || strings.TrimSpace(sc.Text()) != header {
		return nil, fmt.Errorf("%w: expected %q", ErrBadCoeffFile, header)
	}

	out := make([]float64, 0, count)
	for len(out) < count {
		if !sc.Scan() {
			return nil, fmt.Errorf("%w: block %q is truncated", ErrBadCoeffFile, header)
		}
		for _, field := range strings.Fields(sc.Text()) {
			v, err := strconv.ParseFloat(field, 64)
			if err != nil {
				return nil, fmt.Errorf("%w: block %q: %v", ErrBadCoeffFile, header, err)
			}
			out = append(out, v)
		}
	}
	if len(out) != count {
		return nil, fmt.Errorf("%w: block %q has too many values", ErrBadCoeffFile, header)
	}
	return out, nil
}
