package psi

import (
	"bytes"

	"github.com/k-danil/go-astits/v3/internal/bitstest"
)

var tot = &TOT{
	Descriptors: descriptors,
	UTCTime:     dvbTime,
}

func totBytes() []byte {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write(dvbTimeBytes) // UTC time
	_ = w.Write("0000")       // Reserved
	descriptorsBytes(w)       // Service #1 descriptors
	return buf.Bytes()
}
