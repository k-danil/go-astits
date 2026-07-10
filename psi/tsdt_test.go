package psi

import (
	"testing"

	"github.com/k-danil/go-astits/v2/descriptor"
	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTSDTSection(t *testing.T) {
	// one CA descriptor: tag 0x09, len 4, CA_system_id 0x1234, EMM PID 0x0BB8
	bs := []byte{0x09, 0x04, 0x12, 0x34, 0xeb, 0xb8}
	d, err := parseTSDTSection(bytesiter.New(bs), len(bs))
	require.NoError(t, err)
	require.Len(t, d.Descriptors, 1)

	ca, ok := d.Descriptors[0].(*descriptor.CA)
	require.True(t, ok)
	assert.Equal(t, uint16(0x1234), ca.SystemID)
}
