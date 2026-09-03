package demux

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v3/ts"
	"github.com/k-danil/go-astits/v3/tsio"
)

// Cancellation is polled once per window, so a reader that could hand over
// its whole input as one window (a slice) must still see it within the poll
// cadence — not after the last packet, where EOF would win.
func TestCancelObservedWithinWindow(t *testing.T) {
	const packets = 8192
	raw := make([]byte, 0, packets*ts.PacketSize)
	for i := range packets {
		raw = append(raw, payloadPacket(ts.PIDNull, uint8(i)&0xf, false, []byte{0xFF})...)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	seen := 0
	dmx := New(ctx, tsio.NewBytesReader(raw),
		WithPacketSize(ts.PacketSize), WithZeroCopyPackets(4096),
		WithPacketHook(func(*ts.Packet) {
			seen++
			if seen == 10 {
				cancel()
			}
		}))
	defer dmx.Close()

	var err error
	for _, err = range dmx.Events() {
		if err != nil {
			break
		}
	}
	require.True(t, errors.Is(err, context.Canceled), "got %v", err)
	require.Less(t, seen, 10+2*1024, "cancel must be seen within the poll cadence")
}
