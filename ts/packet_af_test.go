package ts

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseInto(t *testing.T, p *Packet, bs []byte) {
	skip, err := p.parse(bs, EmptySkipper, nil)
	require.NoError(t, err)
	require.False(t, skip)
}

func TestPacketEmbeddedAFReuse(t *testing.T) {
	p := NewPacket()
	defer p.Close()

	// Full AF (fixture: PCR/OPCR/private/extension)
	bs1, _ := packet(packetHeader, packetAdaptationField, []byte("payload"), false)
	parseInto(t, p, bs1[:PacketSize])
	require.NotNil(t, p.AdaptationField)
	assert.Same(t, &p.af, p.AdaptationField)
	assert.Equal(t, []byte("test"), p.AdaptationField.TransportPrivateData)
	assert.NotNil(t, p.AdaptationField.AdaptationExtensionField)

	// Packet without AF: the pointer must be reset to nil
	bs2, _ := packetShort(PacketHeader{HasPayload: true, PID: 0x100}, []byte{0xde})
	parseInto(t, p, bs2[:PacketSize])
	assert.Nil(t, p.AdaptationField)

	// Minimal AF (RAI only): stale private/extension from the first parse
	// must not leak through
	minimalAF := make([]byte, PacketSize)
	minimalAF[0] = syncByte
	minimalAF[1] = 0x01
	minimalAF[2] = 0x00 // PID 0x100
	minimalAF[3] = 0x30 // AF+payload, CC=0
	minimalAF[4] = 0x01 // AF length
	minimalAF[5] = 0x40 // RAI only
	parseInto(t, p, minimalAF)
	require.NotNil(t, p.AdaptationField)
	assert.True(t, p.AdaptationField.RandomAccessIndicator)
	assert.False(t, p.AdaptationField.HasTransportPrivateData)
	assert.Nil(t, p.AdaptationField.TransportPrivateData)
	assert.Nil(t, p.AdaptationField.AdaptationExtensionField)

	// AF with Length=0 (one-byte stuffing): the flags byte is not parsed at all —
	// stale Has-flags of the previous packet used to produce phantom PCRs
	parseInto(t, p, bs1[:PacketSize]) // full AF with PCR again
	require.True(t, p.AdaptationField.HasPCR)
	zeroLenAF := make([]byte, PacketSize)
	zeroLenAF[0] = syncByte
	zeroLenAF[1] = 0x01
	zeroLenAF[2] = 0x00 // PID 0x100
	zeroLenAF[3] = 0x30 // AF+payload, CC=0
	zeroLenAF[4] = 0x00 // AF length = 0
	parseInto(t, p, zeroLenAF)
	require.NotNil(t, p.AdaptationField)
	assert.False(t, p.AdaptationField.HasPCR)
	assert.False(t, p.AdaptationField.RandomAccessIndicator)
}
