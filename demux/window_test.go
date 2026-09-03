package demux

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v3/ts"
)

// The event stream — hook calls included — must not depend on where the read
// windows fall: a packet that queues an error or completes a unit ends the
// window walk, and a packet the walk cannot take goes through the per-packet
// path at the same position.
func TestDemuxerEventsIndependentOfWindow(t *testing.T) {
	pesStart := []byte{0, 0, 1, 0xe0, 0, 0, 0x80, 0, 0}
	var raw []byte
	for i := range 40 {
		cc := uint8(i)
		raw = append(raw, payloadPacket(0x100, cc&0xf, true, append(pesStart, byte(i)))...)
		raw = append(raw, payloadPacket(0x200, cc&0xf, true, append(pesStart, byte(i)))...)
		raw = append(raw, payloadPacket(0x100, (cc+1)&0xf, false, []byte{1, 2, 3})...)
		if i%3 == 0 {
			raw = append(raw, corruptAlignedPacket()...) // per-packet path
		}
		if i%4 == 0 {
			raw = append(raw, payloadPacket(0x200, (cc+3)&0xf, false, []byte{9})...) // CC gap: torn unit
		}
	}

	trace := func(window uint) []string {
		var out []string
		dmx := New(context.Background(), bytes.NewReader(raw),
			WithPacketSize(ts.PacketSize), WithSkipErrLimit(-1), WithRecoverableErrors(),
			WithZeroCopyPackets(window),
			WithPacketHook(func(p *ts.Packet) {
				out = append(out, fmt.Sprintf("hook pid=%d off=%d cc=%d", p.Header.PID, p.Offset, p.Header.ContinuityCounter))
			}))
		defer dmx.Close()
		for ev, err := range dmx.Events() {
			if err != nil {
				var re *ts.RecoverableError
				require.ErrorAs(t, err, &re, "window %d: %v", window, err)
				out = append(out, fmt.Sprintf("err %s pid=%d off=%d dropped=%d", re.Kind, re.PID, re.Offset, re.Dropped))
				continue
			}
			if ev == EventPES {
				d := dmx.PES()
				out = append(out, fmt.Sprintf("pes pid=%d first=%d last=%d len=%d", d.PID, d.FirstPacketOffset, d.LastPacketOffset, len(d.Data.Data)))
			}
		}
		return out
	}

	reference := trace(0)
	require.NotEmpty(t, reference)
	for _, window := range []uint{1, 2, 3, 7, 4096} {
		require.Equal(t, reference, trace(window), "window of %d packets", window)
	}
}
