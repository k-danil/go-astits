package demux

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v3/ts"
)

// Raw() must return the same bytes in copy and zero-copy view modes: the
// view path repoints p.raw at the batch buffer instead of the owned array.
func TestZeroCopyRawMatchesCopy(t *testing.T) {
	stream := offsetTestStream([]uint16{0x100, 0x101, 0x102, 0x103, 0x104})

	walk := func(zeroCopy bool) (raws [][]byte) {
		opts := []func(*Demuxer){WithPacketSize(ts.PacketSize)}
		if zeroCopy {
			opts = append(opts, WithZeroCopyPackets(2))
		}
		dmx := New(context.Background(), bytes.NewReader(stream), opts...)
		p := ts.NewPacket()
		defer p.Close()
		for {
			if err := dmx.NextPacketTo(p); err != nil {
				return
			}
			raws = append(raws, bytes.Clone(p.Raw()))
		}
	}

	copyRaws := walk(false)
	viewRaws := walk(true)
	require.NotEmpty(t, copyRaws)
	require.Len(t, viewRaws, len(copyRaws))
	for i := range copyRaws {
		require.Len(t, copyRaws[i], ts.PacketSize)
		assert.Equal(t, copyRaws[i], viewRaws[i], "packet %d", i)
	}
}

// A view-mode PID rewrite via UpdateHeader must land in Raw() (the batch view),
// so muxer passthrough of Raw() reflects the new PID.
func TestZeroCopyRawReflectsUpdateHeader(t *testing.T) {
	stream := offsetTestStream([]uint16{0x100})
	dmx := New(context.Background(), bytes.NewReader(stream),
		WithPacketSize(ts.PacketSize), WithZeroCopyPackets(4))
	p := ts.NewPacket()
	defer p.Close()

	require.NoError(t, dmx.NextPacketTo(p))
	p.Header.PID = 0x1ff
	p.UpdateHeader()

	var parsed ts.Packet
	_, err := parsed.Header.Parse(p.Raw())
	require.NoError(t, err)
	assert.Equal(t, uint16(0x1ff), parsed.Header.PID)
}
