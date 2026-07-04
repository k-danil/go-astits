package ts

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/k-danil/go-astits/v2/internal/bitstest"
)

var ptsClockReference = NewClockReference(5726623061, 0)

func ptsBytes(flag string) []byte {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write(flag)              // Flag
	_ = w.Write("101")             // 32...30
	_ = w.Write("1")               // Dummy
	_ = w.Write("010101010101010") // 29...15
	_ = w.Write("1")               // Dummy
	_ = w.Write("101010101010101") // 14...0
	_ = w.Write("1")               // Dummy
	return buf.Bytes()
}

var dtsClockReference = NewClockReference(5726623060, 0)

func dtsBytes(flag string) []byte {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write(flag)              // Flag
	_ = w.Write("101")             // 32...30
	_ = w.Write("1")               // Dummy
	_ = w.Write("010101010101010") // 29...15
	_ = w.Write("1")               // Dummy
	_ = w.Write("101010101010100") // 14...0
	_ = w.Write("1")               // Dummy
	return buf.Bytes()
}

func escrBytes() []byte {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write("11")              // Dummy
	_ = w.Write("011")             // 32...30
	_ = w.Write("1")               // Dummy
	_ = w.Write("000010111110000") // 29...15
	_ = w.Write("1")               // Dummy
	_ = w.Write("000010111001111") // 14...0
	_ = w.Write("1")               // Dummy
	_ = w.Write("000111010")       // Ext
	_ = w.Write("1")               // Dummy
	return buf.Bytes()
}

func TestParsePTSOrDTS(t *testing.T) {
	var v ClockReference
	n, err := v.ParsePTSDTS(ptsBytes("0010"))
	assert.Equal(t, PTSDTSSize, n)
	assert.Equal(t, v, ptsClockReference)
	assert.NoError(t, err)
}

func TestWritePTSOrDTS(t *testing.T) {
	bs := make([]byte, PTSDTSSize)
	n := dtsClockReference.PutPTSDTS(bs, uint8(0b0010))
	assert.Equal(t, n, 5)
	assert.Equal(t, dtsBytes("0010"), bs)
}

func TestParseESCR(t *testing.T) {
	var v ClockReference
	_, err := v.ParseESCR(escrBytes())
	assert.Equal(t, v, clockReference)
	assert.NoError(t, err)
}

func TestWriteESCR(t *testing.T) {
	bs := make([]byte, ESCRSize)
	n := clockReference.PutESCR(bs)
	assert.Equal(t, n, 6)
	assert.Equal(t, escrBytes(), bs)
}
