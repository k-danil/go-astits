package ts

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/k-danil/go-astits/v2/internal/bitstest"
)

func TestAutoDetectPacketSize(t *testing.T) {
	// Packet should start with a sync byte
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write(uint8(2))
	_ = w.Write(syncByte)
	_, err := autoDetectPacketSize(bytes.NewReader(buf.Bytes()))
	assert.EqualError(t, err, ErrPacketMustStartWithASyncByte.Error())

	// Valid packet size
	buf.Reset()
	_ = w.Write(syncByte)
	_ = w.Write(make([]byte, 20))
	_ = w.Write(syncByte)
	_ = w.Write(make([]byte, 166))
	_ = w.Write(syncByte)
	_ = w.Write(make([]byte, 187))
	_ = w.Write([]byte("test"))
	r := bytes.NewReader(buf.Bytes())
	p, err := autoDetectPacketSize(r)
	assert.NoError(t, err)
	assert.Equal(t, uint(PacketSize), p)
	assert.Equal(t, 380, r.Len())
}
