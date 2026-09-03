package ts

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The extension's descriptors view the packet buffer after parse; the copy
// must own them, or the next read into that buffer rewrites the retained AF.
func TestAdaptationFieldCopyFromOwnsAFDescriptors(t *testing.T) {
	bs := make([]byte, PacketSize)
	bs[0] = syncByte
	bs[1], bs[2] = 0x01, 0x00   // PID 0x100
	bs[3] = 0x30                // AF + payload, CC 0
	bs[4] = 0x08                // AF length
	bs[5] = 0x03                // private data + extension flags
	bs[6] = 0x02                // private data length
	bs[7], bs[8] = 0x11, 0x22   // the private data
	bs[9] = 0x03                // extension length
	bs[10] = 0x00               // af_descriptor_not_present_flag == 0
	bs[11], bs[12] = 0xaa, 0xbb // the descriptors

	var p Packet
	skip, err := p.parse(bs, nil, nil)
	require.NoError(t, err)
	require.False(t, skip)
	require.NotNil(t, p.AdaptationField.AdaptationExtensionField)
	require.Equal(t, []byte{0xaa, 0xbb}, p.AdaptationField.AdaptationExtensionField.AFDescriptors)

	require.Equal(t, []byte{0x11, 0x22}, p.AdaptationField.TransportPrivateData)

	var cp PacketAdaptationField
	cp.CopyFrom(&p.AdaptationField)
	bs[7], bs[8] = 0x00, 0x00
	bs[11], bs[12] = 0x00, 0x00
	assert.Equal(t, []byte{0x11, 0x22}, cp.TransportPrivateData, "private data must be owned, not a view into the read buffer")
	assert.Equal(t, []byte{0xaa, 0xbb}, cp.AdaptationExtensionField.AFDescriptors)
}

func TestSetAdaptationFieldDoesNotWriteReadWindow(t *testing.T) {
	win := make([]byte, 2*PacketSize)
	first := win[:PacketSize]
	first[0] = syncByte
	first[1], first[2] = 0x01, 0x00 // PID 0x100
	first[3] = 0x30                 // AF + payload
	first[4] = 0x04                 // AF length
	first[5] = 0x02                 // transport_private_data_flag
	first[6] = 0x02                 // private data length
	first[7], first[8] = 0x11, 0x22
	second := win[PacketSize:]
	second[0] = syncByte
	second[1], second[2] = 0x01, 0x01
	second[3] = 0x10
	for i := HeaderSize; i < len(second); i++ {
		second[i] = 0x5a
	}
	untouched := bytes.Clone(win)

	var p Packet
	skip, err := p.ParseAt(first, 0, nil, nil)
	require.NoError(t, err)
	require.False(t, skip)
	require.Equal(t, []byte{0x11, 0x22}, p.AdaptationField.TransportPrivateData)

	priv := bytes.Repeat([]byte{0xde}, 200)
	p.SetAdaptationField(&PacketAdaptationField{
		HasTransportPrivateData:    true,
		TransportPrivateDataLength: uint8(len(priv)),
		TransportPrivateData:       priv,
	})

	assert.Equal(t, untouched, win, "the read window is untouched")
	assert.Equal(t, priv, p.AdaptationField.TransportPrivateData)
}
