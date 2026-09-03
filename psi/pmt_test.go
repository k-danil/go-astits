package psi

import (
	"bytes"
	"testing"

	"github.com/k-danil/go-astits/v3/internal/bitstest"
	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

var pmt = &PMT{
	ElementaryStreams: []ElementaryStream{{
		ElementaryPID:               2730,
		ElementaryStreamDescriptors: descriptors,
		StreamType:                  StreamTypeMPEG1Audio,
	}},
	PCRPID:             5461,
	ProgramDescriptors: descriptors,
	ProgramNumber:      1,
}

func pmtBytes() []byte {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write("111")                       // Reserved bits
	_ = w.Write("1010101010101")             // PCR PID
	_ = w.Write("1111")                      // Reserved
	descriptorsBytes(w)                      // Program descriptors
	_ = w.Write(uint8(StreamTypeMPEG1Audio)) // Stream #1 stream type
	_ = w.Write("111")                       // Stream #1 reserved
	_ = w.Write("0101010101010")             // Stream #1 PID
	_ = w.Write("1111")                      // Stream #1 reserved
	descriptorsBytes(w)                      // Stream #1 descriptors
	return buf.Bytes()
}

func BenchmarkParsePMTSection(b *testing.B) {
	b.ReportAllocs()
	bs := pmtBytes()

	for i := 0; i < b.N; i++ {
		_, _ = parsePMTSection(bytesiter.New(bs), len(bs), uint16(1))
	}
}

func BenchmarkWritePMTSection(b *testing.B) {
	b.ReportAllocs()

	dst := make([]byte, 0, 1024)

	for i := 0; i < b.N; i++ {
		dst = pmt.appendSection(dst[:0])
	}
}
