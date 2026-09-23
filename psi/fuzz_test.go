package psi

import (
	"bytes"
	"math/rand/v2"
	"testing"

	"github.com/k-danil/go-astits/v3/internal/bitstest"
)

func FuzzParse(f *testing.F) {
	f.Add(psiBytes())
	for _, tc := range psiDataTestCases {
		buf := bytes.Buffer{}
		w := bitstest.NewWriter(&buf)
		tc.bytesFunc(w)
		f.Add(buf.Bytes())
	}
	r := rand.New(rand.NewPCG(3, 4))
	for _, tc := range satGenerators {
		_, d := randSATData(r, tc.gen)
		b, err := d.Append(nil)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, bs []byte) {
		_, _ = Parse(bs)
	})
}
