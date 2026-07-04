package demux

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/ts"
)

func offsetTestStream(pids []uint16) []byte {
	buf := &bytes.Buffer{}
	cc := make(map[uint16]uint8)
	for _, pid := range pids {
		b, _ := packetShort(ts.PacketHeader{
			ContinuityCounter: cc[pid],
			HasPayload:        true,
			PID:               pid,
		}, []byte{0xde, 0xad, 0xbe, 0xef})
		cc[pid] = (cc[pid] + 1) & 0xf
		// packetShort pads without accounting for the header and returns 192 bytes
		buf.Write(b[:ts.PacketSize])
	}
	return buf.Bytes()
}

func TestPacketOffset(t *testing.T) {
	const keepPID, skipPID = 0x100, 0x101
	pids := []uint16{keepPID, skipPID, skipPID, keepPID, skipPID, keepPID}
	stream := offsetTestStream(pids)

	tests := []struct {
		name        string
		skipper     ts.PacketSkipper
		wantOffsets []int64
	}{
		{"noSkipper", nil, []int64{0, 188, 376, 564, 752, 940}},
		{
			"skipperAdvancesOffset",
			func(p *ts.Packet) bool { return p.Header.PID == skipPID },
			[]int64{0, 564, 940},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := []func(*Demuxer){WithPacketSize(ts.PacketSize)}
			if tc.skipper != nil {
				opts = append(opts, WithPacketSkipper(tc.skipper))
			}
			dmx := New(context.Background(), bytes.NewReader(stream), opts...)

			p := ts.NewPacket()
			defer p.Close()
			var offsets []int64
			for {
				if err := dmx.NextPacketTo(p); err != nil {
					require.True(t, errors.Is(err, ts.ErrNoMorePackets))
					break
				}
				offsets = append(offsets, p.Offset)
			}
			assert.Equal(t, tc.wantOffsets, offsets)
		})
	}
}
