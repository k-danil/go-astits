package descriptor

import (
	"bytes"
	"testing"

	"github.com/k-danil/go-astits/v2/internal/bitstest"
)

func FuzzParse(f *testing.F) {
	for _, tc := range descriptorTestTable {
		buf := bytes.Buffer{}
		buf.Write([]byte{0x00, 0x00}) // reserve two bytes for length
		w := bitstest.NewWriter(&buf)
		tc.bytesFunc(w)
		bs := buf.Bytes()
		l := len(bs) - 2
		bs[0] = byte(l>>8) | 0xf0
		bs[1] = byte(l)
		f.Add(bs)
	}
	f.Fuzz(func(t *testing.T, bs []byte) {
		Parse(bs)
	})
}
