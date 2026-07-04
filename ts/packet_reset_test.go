package ts

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPacketResetKeepsParseCorrect(t *testing.T) {
	p := NewPacket()
	defer p.Close()
	for i := range p.bs {
		p.bs[i] = 0xa5
	}
	p.raw = p.bs[:]
	p.Offset = 42

	p.Reset()
	assert.Nil(t, p.raw)
	assert.Equal(t, int64(0), p.Offset)
	assert.Equal(t, PacketHeader{}, p.Header)
	assert.Nil(t, p.AdaptationField)
	assert.Nil(t, p.Payload)

	b, _ := packetShort(PacketHeader{HasPayload: true, PID: 0x100}, []byte{0xde, 0xad, 0xbe, 0xef})
	pb, err := NewPacketBuffer(bytes.NewReader(b[:PacketSize]), PacketSize, 0, EmptySkipper, 0)
	require.NoError(t, err)
	require.NoError(t, pb.Next(p))
	assert.Equal(t, uint16(0x100), p.Header.PID)
	assert.Equal(t, []byte{0xde, 0xad, 0xbe, 0xef}, p.Payload[:4])
}
