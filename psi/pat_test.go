package psi

import (
	"bytes"
	"testing"

	"github.com/k-danil/go-astits/v3/internal/bitstest"
	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

var pat = &PAT{
	Programs: []PATProgram{
		{ProgramMapID: 3, ProgramNumber: 2},
		{ProgramMapID: 5, ProgramNumber: 4},
	},
	TransportStreamID: 1,
}

func patBytes() []byte {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write(uint16(2))       // Program #1 number
	_ = w.Write("111")           // Program #1 reserved bits
	_ = w.Write("0000000000011") // Program #1 map ID
	_ = w.Write(uint16(4))       // Program #2 number
	_ = w.Write("111")           // Program #2 reserved bits
	_ = w.Write("0000000000101") // Program #3 map ID
	return buf.Bytes()
}

func BenchmarkParsePATSection(b *testing.B) {
	b.ReportAllocs()
	bs := patBytes()

	for i := 0; i < b.N; i++ {
		_, _ = parsePATSection(bytesiter.New(bs), len(bs), uint16(1))
	}
}

func BenchmarkWritePATSection(b *testing.B) {
	b.ReportAllocs()

	dst := make([]byte, 0, 1024)

	for i := 0; i < b.N; i++ {
		dst = pat.appendSection(dst[:0])
	}
}
