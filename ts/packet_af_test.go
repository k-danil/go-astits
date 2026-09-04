package ts

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseInto(t *testing.T, p *Packet, bs []byte) {
	skip, err := p.parse(bs, nil, nil)
	require.NoError(t, err)
	require.False(t, skip)
}

// A field longer than its length byte can express is refused before anything
// is written: a truncated length declares less than the body that follows it.
func TestAdaptationFieldPutRejectsOverflow(t *testing.T) {
	for _, tc := range []struct {
		name string
		af   PacketAdaptationField
	}{
		{"private data past the length byte", PacketAdaptationField{HasTransportPrivateData: true, TransportPrivateData: make([]byte, 0xff)}},
		{"stuffing past the length byte", PacketAdaptationField{StuffingLength: 0xff}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bs := make([]byte, 2*PacketSize)
			n, err := tc.af.Put(bs)
			require.ErrorIs(t, err, ErrAdaptationFieldOverflow)
			assert.Zero(t, n)
		})
	}
}

func TestAdaptationFieldPutRejectsContradiction(t *testing.T) {
	for _, tc := range []struct {
		name string
		af   PacketAdaptationField
	}{
		{"extension flag without the extension", PacketAdaptationField{HasAdaptationExtensionField: true}},
		{"one-byte stuffing carrying a PCR", PacketAdaptationField{IsOneByteStuffing: true, HasPCR: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bs := make([]byte, PacketSize)
			n, err := tc.af.Put(bs)
			require.ErrorIs(t, err, ErrContradictoryAdaptationField)
			assert.Zero(t, n)
		})
	}
}

// The extension body is bounded by its declared length: a field reaching past
// it must fail instead of reading the adaptation field's stuffing.
func TestAdaptationExtensionStopsAtDeclaredLength(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []byte
	}{
		{"ltw declared past the extension", []byte{0x03, 0x01, 0x01, 0x80}},
		{"extension without its flags byte", []byte{0x02, 0x01, 0x00}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bs := bytes.Repeat([]byte{0xff}, PacketSize-HeaderSize)
			copy(bs, tc.body)
			var af PacketAdaptationField
			_, err := af.Parse(bs)
			require.ErrorIs(t, err, ErrShortPacket)
			if af.AdaptationExtensionField != nil {
				assert.Zero(t, af.AdaptationExtensionField.LegalTimeWindowOffset, "stuffing must not be read as a legal time window")
			}
		})
	}
}

// A one-byte adaptation field (length 0) is stuffing, not a field with a flags
// byte: writing that byte back would push the payload out of the packet.
func TestPacketOneByteAdaptationFieldRoundTrip(t *testing.T) {
	in := make([]byte, PacketSize)
	in[0] = syncByte
	in[1] = 0x01
	in[2] = 0x00 // PID 0x100
	in[3] = 0x30 // AF + payload, CC 0
	in[4] = 0x00 // adaptation_field_length
	for i := HeaderSize + 1; i < PacketSize; i++ {
		in[i] = byte(i)
	}

	p := NewPacket()
	defer p.Close()
	parseInto(t, p, in)
	require.Len(t, p.Payload, PacketSize-HeaderSize-1)

	out := make([]byte, PacketSize)
	n, err := p.Put(out)
	require.NoError(t, err)
	require.Equal(t, PacketSize, n)
	assert.Equal(t, in, out)
}

func TestPacketEmbeddedAFReuse(t *testing.T) {
	p := NewPacket()
	defer p.Close()

	// Full AF (fixture: PCR/OPCR/private/extension)
	bs1, _ := packet([]byte("payload"), false)
	parseInto(t, p, bs1[:PacketSize])
	require.True(t, p.Header.HasAdaptationField)
	assert.Equal(t, []byte("test"), p.AdaptationField.TransportPrivateData)
	assert.NotNil(t, p.AdaptationField.AdaptationExtensionField)

	// Packet without AF: the pointer must be reset to nil
	bs2, _ := packetShort(PacketHeader{HasPayload: true, PID: 0x100}, []byte{0xde})
	parseInto(t, p, bs2[:PacketSize])
	assert.False(t, p.Header.HasAdaptationField)

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
	require.True(t, p.Header.HasAdaptationField)
	assert.True(t, p.AdaptationField.RandomAccessIndicator)
	assert.False(t, p.AdaptationField.HasTransportPrivateData)
	assert.Nil(t, p.AdaptationField.TransportPrivateData)
	assert.Nil(t, p.AdaptationField.AdaptationExtensionField)

	parseInto(t, p, bs1[:PacketSize]) // full AF with PCR again
	require.True(t, p.AdaptationField.HasPCR)
	zeroLenAF := make([]byte, PacketSize)
	zeroLenAF[0] = syncByte
	zeroLenAF[1] = 0x01
	zeroLenAF[2] = 0x00 // PID 0x100
	zeroLenAF[3] = 0x30 // AF+payload, CC=0
	zeroLenAF[4] = 0x00 // AF length = 0
	parseInto(t, p, zeroLenAF)
	require.True(t, p.Header.HasAdaptationField)
	assert.False(t, p.AdaptationField.HasPCR)
	assert.False(t, p.AdaptationField.RandomAccessIndicator)
}
