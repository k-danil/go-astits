package psi

import (
	"testing"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRSTSection(t *testing.T) {
	bs := []byte{
		0x00, 0x01, 0x00, 0x02, 0x00, 0x03, 0x00, 0x04, 0xf4, // tsid,onid,svc,event, running=4
		0x00, 0x05, 0x00, 0x06, 0x00, 0x07, 0x00, 0x08, 0xf1, // running=1
	}
	d, err := parseRSTSection(bytesiter.New(bs), len(bs))
	require.NoError(t, err)
	require.Len(t, d.Events, 2)

	assert.Equal(t, RSTEvent{
		TransportStreamID: 0x0001, OriginalNetworkID: 0x0002,
		ServiceID: 0x0003, EventID: 0x0004, RunningStatus: 4,
	}, d.Events[0])
	assert.Equal(t, RSTEvent{
		TransportStreamID: 0x0005, OriginalNetworkID: 0x0006,
		ServiceID: 0x0007, EventID: 0x0008, RunningStatus: 1,
	}, d.Events[1])
}
