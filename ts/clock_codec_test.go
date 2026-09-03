package ts

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v3/internal/bitstest"
)

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

var clockReference = NewClockReference(3271034319, 58)

func TestParseESCR(t *testing.T) {
	var v ClockReference
	_, err := v.ParseESCR(escrBytes())
	assert.Equal(t, v, clockReference)
	assert.NoError(t, err)
}

func TestWriteESCR(t *testing.T) {
	bs := make([]byte, ESCRSize)
	n := clockReference.PutESCR(bs)
	assert.Equal(t, 6, n)
	assert.Equal(t, escrBytes(), bs)
}

// A backwards Diff must fold into the 33-bit base before it can be written.
func TestNegativeClockReferenceOnTheWire(t *testing.T) {
	d := NewClockReference(1000, 0).Diff(NewClockReference(2000, 0))
	assert.Less(t, d.Base(), uint64(1)<<33, "base is a 33-bit field")
	assert.Less(t, d.Extension(), uint64(PTSTicks))

	bs := make([]byte, PCRSize)
	require.Equal(t, PCRSize, d.PutPCR(bs))
	var parsed ClockReference
	n, err := parsed.ParsePCR(bs)
	require.NoError(t, err)
	require.Equal(t, PCRSize, n)
	assert.Zero(t, parsed.Diff(d), "the wire form names the same instant")
}
