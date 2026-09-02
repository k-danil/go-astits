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
	bs[1], bs[2] = 0x01, 0x00 // PID 0x100
	bs[3] = 0x30              // AF + payload, CC 0
	bs[4] = 0x05              // AF length
	bs[5] = 0x01              // extension flag only
	bs[6] = 0x03              // extension length
	bs[7] = 0x00              // af_descriptor_not_present_flag == 0
	bs[8], bs[9] = 0xaa, 0xbb // the descriptors

	var p Packet
	skip, err := p.parse(bs, nil, nil)
	require.NoError(t, err)
	require.False(t, skip)
	require.NotNil(t, p.AdaptationField.AdaptationExtensionField)
	require.Equal(t, []byte{0xaa, 0xbb}, p.AdaptationField.AdaptationExtensionField.AFDescriptors)

	var cp PacketAdaptationField
	cp.CopyFrom(&p.AdaptationField)
	bs[8], bs[9] = 0x00, 0x00
	assert.Equal(t, []byte{0xaa, 0xbb}, cp.AdaptationExtensionField.AFDescriptors)
}
