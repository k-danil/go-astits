package ts

import (
	"bytes"
	"testing"

	"github.com/asticode/go-astikit"
	"github.com/stretchr/testify/assert"
)

var ptsClockReference = NewClockReference(5726623061, 0)

func ptsBytes(flag string) []byte {
	buf := &bytes.Buffer{}
	w := astikit.NewBitsWriter(astikit.BitsWriterOptions{Writer: buf})
	w.Write(flag)              // Flag
	w.Write("101")             // 32...30
	w.Write("1")               // Dummy
	w.Write("010101010101010") // 29...15
	w.Write("1")               // Dummy
	w.Write("101010101010101") // 14...0
	w.Write("1")               // Dummy
	return buf.Bytes()
}

var dtsClockReference = NewClockReference(5726623060, 0)

func dtsBytes(flag string) []byte {
	buf := &bytes.Buffer{}
	w := astikit.NewBitsWriter(astikit.BitsWriterOptions{Writer: buf})
	w.Write(flag)              // Flag
	w.Write("101")             // 32...30
	w.Write("1")               // Dummy
	w.Write("010101010101010") // 29...15
	w.Write("1")               // Dummy
	w.Write("101010101010100") // 14...0
	w.Write("1")               // Dummy
	return buf.Bytes()
}

func escrBytes() []byte {
	buf := &bytes.Buffer{}
	w := astikit.NewBitsWriter(astikit.BitsWriterOptions{Writer: buf})
	w.Write("11")              // Dummy
	w.Write("011")             // 32...30
	w.Write("1")               // Dummy
	w.Write("000010111110000") // 29...15
	w.Write("1")               // Dummy
	w.Write("000010111001111") // 14...0
	w.Write("1")               // Dummy
	w.Write("000111010")       // Ext
	w.Write("1")               // Dummy
	return buf.Bytes()
}

func TestParsePTSOrDTS(t *testing.T) {
	var v ClockReference
	err := v.parsePTSOrDTS(astikit.NewBytesIterator(ptsBytes("0010")))
	assert.Equal(t, v, ptsClockReference)
	assert.NoError(t, err)
}

func TestWritePTSOrDTS(t *testing.T) {
	bs := make([]byte, PTSOrDTSByteLength)
	n := dtsClockReference.PutPTSOrDTSBytes(bs, uint8(0b0010))
	assert.Equal(t, n, 5)
	assert.Equal(t, dtsBytes("0010"), bs)
}

func TestParseESCR(t *testing.T) {
	var v ClockReference
	_, err := v.ParseESCRBytes(escrBytes(), 0)
	assert.Equal(t, v, clockReference)
	assert.NoError(t, err)
}

func TestWriteESCR(t *testing.T) {
	bs := make([]byte, ESCRByteLength)
	n := clockReference.PutESCRBytes(bs)
	assert.Equal(t, n, 6)
	assert.Equal(t, escrBytes(), bs)
}
