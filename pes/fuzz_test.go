package pes

import (
	"bytes"
	"testing"

	"github.com/k-danil/go-astits/v2/internal/bitstest"
)

func FuzzParse(f *testing.F) {
	for _, tc := range pesTestCases {
		buf := bytes.Buffer{}
		w := bitstest.NewWriter(&buf)
		tc.headerBytesFunc(w, true, true)
		tc.optionalHeaderBytesFunc(w, true, true)
		tc.bytesFunc(w, true, true)
		f.Add(buf.Bytes())
	}
	f.Fuzz(func(t *testing.T, bs []byte) {
		var d Data
		d.Parse(bs)
	})
}
