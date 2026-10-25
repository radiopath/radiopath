package hf

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
)

// ionMap is a monthly map of foF2 or M(3000)F2 on a 1.5 degree grid for the
// 24 hours and for the sunspot numbers 0 and 100. The values are kept in the
// order of the file (float32 like the C arrays).
type ionMap struct {
	data []float32
}

// at returns the value for hour i, longitude index j, latitude index k and
// SSN index m (0 = SSN 0, 1 = SSN 100) as a double, like the C array
// foF2[i][j][k][m].
func (m *ionMap) at(i, j, k, s int) float64 {
	return float64(m.data[(s*(ionLng*ionLat*ionHours))+(j*(ionLat*ionHours))+(k*ionHours)+i])
}

// readIonParametersBin reads the maps of a month (0-11) from ionosNN.bin in
// dir. The file is a Fortran unformatted file with two records: foF2 and
// M(3000)F2, each 24 x 121 x 241 x 2 float32 values.
func readIonParametersBin(month int, dir string) (foF2, m3kF2 *ionMap, err error) {
	numfoF2 := ionHours * ionLng * ionLat * ionSSN
	name := filepath.Join(dir, fmt.Sprintf("ionos%02d.bin", month+1))
	fp, err := os.Open(name)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrReadIonParameters, err)
	}
	defer fp.Close()

	// The first 5 bytes of the file are overhead that Fortran puts in.
	if _, err = io.CopyN(io.Discard, fp, 5); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrReadIonParameters, err)
	}
	foF2 = &ionMap{data: make([]float32, numfoF2)}
	if err = readFloat32s(fp, foF2.data); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrReadIonParameters, err)
	}

	// The tail of the foF2 record and the header of the M(3000)F2 record.
	if _, err = io.CopyN(io.Discard, fp, 10); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrReadIonParameters, err)
	}
	m3kF2 = &ionMap{data: make([]float32, numfoF2)}
	if err = readFloat32s(fp, m3kF2.data); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrReadIonParameters, err)
	}
	return foF2, m3kF2, nil
}

// readFloat32s fills dst with little-endian float32 values from r.
func readFloat32s(r io.Reader, dst []float32) error {
	buf := make([]byte, 4*len(dst))
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}
	for i := range dst {
		dst[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[4*i:]))
	}
	return nil
}
