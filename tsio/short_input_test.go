package tsio_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v3/demux"
	"github.com/k-danil/go-astits/v3/ts"
	"github.com/k-danil/go-astits/v3/tsio"
)

// A slice shorter than the sync-scan window still locks: BytesReader has no
// peek ceiling, so it must not be held to the buffer-size floor.
func TestBytesReaderShortInputSyncLock(t *testing.T) {
	const packets = 4
	data := make([]byte, 0, packets*ts.PacketSize)
	for cc := range packets {
		p := make([]byte, ts.PacketSize)
		p[0], p[1], p[2], p[3] = 0x47, 0x01, 0x00, 0x10|byte(cc)
		for i := ts.HeaderSize; i < len(p); i++ {
			p[i] = 0xFF
		}
		data = append(data, p...)
	}

	dmx := demux.New(context.Background(), tsio.NewBytesReader(data),
		demux.WithSyncLock(), demux.WithPacketSize(ts.PacketSize))
	defer dmx.Close()
	p := ts.NewPacket()
	defer p.Close()
	for i := range packets {
		require.NoError(t, dmx.NextPacketTo(p), "packet %d", i)
		assert.Equal(t, uint16(0x100), p.Header.PID)
		assert.Equal(t, uint8(i), p.Header.ContinuityCounter)
	}
	assert.True(t, errors.Is(dmx.NextPacketTo(p), ts.ErrNoMorePackets))
}
