package psi

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/k-danil/go-astits/v2/internal/bitstest"
	"github.com/k-danil/go-astits/v2/internal/bytesiter"
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

func TestParseTOTSection(t *testing.T) {
	d, err := parseTOTSection(bytesiter.New(totBytes()))
	assert.Equal(t, d, tot)
	assert.NoError(t, err)
}
