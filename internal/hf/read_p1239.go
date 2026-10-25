package hf

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// decileFactors holds the within-the-month foF2 variation of ITU-R P.1239-3
// Tables 2 and 3, indexed [season][hour][latitude/5][ssn range][decile].
type decileFactors [3][24][19][3][2]float64

// readP1239 reads 'P1239-3 Decile Factors.txt' from dir.
func readP1239(dir string) (*decileFactors, error) {
	name := filepath.Join(dir, "P1239-3 Decile Factors.txt")
	fp, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOpenP1239File, err)
	}
	defer fp.Close()

	var f decileFactors
	sc := bufio.NewScanner(fp)
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	nextLine := func() (string, bool) {
		if !sc.Scan() {
			return "", false
		}
		return sc.Text(), true
	}

	const (
		seasons = 3
		lats    = 19 // 0, 5, ... 90 degrees
		ssns    = 3  // R12 < 50, 50 <= R12 <= 100, R12 > 100
		deciles = 2  // lower and upper
	)

	// Two title lines.
	nextLine()
	nextLine()

	for n := 0; n < deciles; n++ {
		for i := 0; i < seasons; i++ {
			for m := 0; m < ssns; m++ {
				// Four lines of header text per table.
				for h := 0; h < 4; h++ {
					nextLine()
				}
				// The latitudes are listed from 90 down to 0 degrees.
				for k := lats - 1; k >= 0; k-- {
					line, ok := nextLine()
					if !ok {
						return nil, ErrNotP12393
					}
					// One latitude label and 24 values, one per hour.
					fields := strings.Fields(line)
					if len(fields) < 25 {
						return nil, ErrNotP12393
					}
					for h := 0; h < 24; h++ {
						v, err := strconv.ParseFloat(fields[h+1], 64)
						if err != nil {
							return nil, ErrNotP12393
						}
						f[i][h][k][m][n] = v
					}
				}
			}
		}
	}
	return &f, nil
}
