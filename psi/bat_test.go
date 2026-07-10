package psi

import (
	"testing"

	"github.com/k-danil/go-astits/v2/descriptor"
	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBATSection(t *testing.T) {
	bs := []byte{
		0xf0, 0x06, 0x09, 0x04, 0x12, 0x34, 0xeb, 0xb8, // bouquet descriptors: one CA
		0xf0, 0x06, // transport_stream_loop_length = 6
		0x00, 0x01, 0x00, 0x02, 0xf0, 0x00, // TS: tsid 1, onid 2, no descriptors
	}
	d, err := parseBATSection(bytesiter.New(bs), 0x1234)
	require.NoError(t, err)

	assert.Equal(t, uint16(0x1234), d.BouquetID)
	require.Len(t, d.BouquetDescriptors, 1)
	ca, ok := d.BouquetDescriptors[0].(*descriptor.CA)
	require.True(t, ok)
	assert.Equal(t, uint16(0x1234), ca.SystemID)

	require.Len(t, d.TransportStreams, 1)
	assert.Equal(t, uint16(0x0001), d.TransportStreams[0].TransportStreamID)
	assert.Equal(t, uint16(0x0002), d.TransportStreams[0].OriginalNetworkID)
}
