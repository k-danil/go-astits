package ts

import (
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
