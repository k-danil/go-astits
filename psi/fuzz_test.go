package psi

import (
	"bytes"
	"testing"

	"github.com/k-danil/go-astits/v2/internal/bitstest"
)

func FuzzParse(f *testing.F) {
	f.Add(psiBytes())
	for _, tc := range psiDataTestCases {
		buf := bytes.Buffer{}
		w := bitstest.NewWriter(&buf)
		tc.bytesFunc(w)
		f.Add(buf.Bytes())
	}
	f.Fuzz(func(t *testing.T, bs []byte) {
		_, _ = Parse(bs)
	})
}
