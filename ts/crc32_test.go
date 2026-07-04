package ts

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	testDataPat = []byte{0x00, 0xb0, 0x0d, 0x00, 0x01, 0xe1, 0x00, 0x00, 0x00, 0x01, 0xf0, 0x00, 0xe2, 0x95, 0xf6, 0x9d}
	testDataPmt = []byte{0x02, 0xb0, 0x1d, 0x00, 0x01, 0xf5, 0x00, 0x00, 0xe1, 0x00, 0xf0, 0x00, 0x1b, 0xe1, 0x00, 0x00,
		0x00, 0x0f, 0xe1, 0x04, 0x00, 0x06, 0x0a, 0x04, 0x72, 0x75, 0x73, 0x00, 0x38, 0x92, 0x85, 0xac}
)

func Test_updateCRC32(t *testing.T) {
	tests := []struct {
		name string
		crc  uint32
		data []byte
	}{
		{
			name: "Calc PAT crc32",
			crc:  binary.BigEndian.Uint32(testDataPat[len(testDataPat)-4:]),
			data: testDataPat[:len(testDataPat)-4],
		}, {
			name: "Calc PMT crc32",
			crc:  binary.BigEndian.Uint32(testDataPmt[len(testDataPmt)-4:]),
			data: testDataPmt[:len(testDataPmt)-4],
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.crc, ComputeCRC32(test.data))
		})
	}
}
