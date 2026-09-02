package ts

import (
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// bytewiseCRC32 is the reference the sliced tables are derived from.
func bytewiseCRC32(crc uint32, bs []byte) uint32 {
	for _, b := range bs {
		crc = (crc << 8) ^ tableCRC32[0][uint8(crc>>24)^b]
	}
	return crc
}

// The sliced update must equal the byte-wise reference for every length
// (whole blocks plus a tail), any state fed in, and across a split.
func TestUpdateCRC32MatchesBytewise(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	data := make([]byte, 6000)
	for i := range data {
		data[i] = byte(rng.Uint32())
	}
	for range 2000 {
		off := rng.IntN(64)
		n := rng.IntN(len(data) - off)
		bs := data[off : off+n]
		seed := rng.Uint32()

		want := bytewiseCRC32(seed, bs)
		require.Equal(t, want, UpdateCRC32(seed, bs), "off=%d n=%d seed=%#x", off, n, seed)

		cut := 0
		if n > 0 {
			cut = rng.IntN(n)
		}
		require.Equal(t, want, UpdateCRC32(UpdateCRC32(seed, bs[:cut]), bs[cut:]), "split off=%d n=%d cut=%d", off, n, cut)
	}
}

var crc32Sink uint32

func BenchmarkUpdateCRC32(b *testing.B) {
	rng := rand.New(rand.NewPCG(3, 4))
	data := make([]byte, 4096)
	for i := range data {
		data[i] = byte(rng.Uint32())
	}
	for _, size := range []int{16, 64, 184, 1024, 4096} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			bs := data[:size]
			b.SetBytes(int64(size))
			var acc uint32
			for b.Loop() {
				acc ^= UpdateCRC32(CRC32Seed, bs)
			}
			crc32Sink = acc
		})
	}
}
