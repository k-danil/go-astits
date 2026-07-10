package psi

import (
	"testing"

	"github.com/k-danil/go-astits/v2/descriptor"
	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSITSection(t *testing.T) {
	bs := []byte{
		0xf0, 0x06, 0x09, 0x04, 0x12, 0x34, 0xeb, 0xb8, // transmission info: one CA descriptor
		0x00, 0x05, // service_id 5
		0xc0, 0x00, // running_status 4, service_loop_length 0
	}
	d, err := parseSITSection(bytesiter.New(bs), len(bs))
	require.NoError(t, err)

	require.Len(t, d.TransmissionInfoDescriptors, 1)
	ca, ok := d.TransmissionInfoDescriptors[0].(*descriptor.CA)
	require.True(t, ok)
	assert.Equal(t, uint16(0x1234), ca.SystemID)

	require.Len(t, d.Services, 1)
	assert.Equal(t, uint16(0x0005), d.Services[0].ServiceID)
	assert.Equal(t, uint8(4), d.Services[0].RunningStatus)
	assert.Empty(t, d.Services[0].Descriptors)
}
