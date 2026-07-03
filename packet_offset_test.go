package astits

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func offsetTestStream(pids []uint16) []byte {
	buf := &bytes.Buffer{}
	cc := make(map[uint16]uint8)
	for _, pid := range pids {
		b, _ := packetShort(PacketHeader{
			ContinuityCounter: cc[pid],
			HasPayload:        true,
			PID:               pid,
		}, []byte{0xde, 0xad, 0xbe, 0xef})
		cc[pid] = (cc[pid] + 1) & 0xf
		// packetShort паддит без учёта хедера и отдаёт 192 байта
		buf.Write(b[:MpegTsPacketSize])
	}
	return buf.Bytes()
}

func TestPacketOffset(t *testing.T) {
	const keepPID, skipPID = 0x100, 0x101
	pids := []uint16{keepPID, skipPID, skipPID, keepPID, skipPID, keepPID}
	stream := offsetTestStream(pids)

	tests := []struct {
		name        string
		skipper     PacketSkipper
		wantOffsets []int64
	}{
		{"noSkipper", nil, []int64{0, 188, 376, 564, 752, 940}},
		{
			"skipperAdvancesOffset",
			func(p *Packet) bool { return p.Header.PID == skipPID },
			[]int64{0, 564, 940},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := []func(*Demuxer){DemuxerOptPacketSize(MpegTsPacketSize)}
			if tc.skipper != nil {
				opts = append(opts, DemuxerOptPacketSkipper(tc.skipper))
			}
			dmx := NewDemuxer(context.Background(), bytes.NewReader(stream), opts...)

			p := NewPacket()
			defer p.Close()
			var offsets []int64
			for {
				if err := dmx.NextPacketTo(p); err != nil {
					require.True(t, errors.Is(err, ErrNoMorePackets))
					break
				}
				offsets = append(offsets, p.Offset)
			}
			assert.Equal(t, tc.wantOffsets, offsets)
		})
	}
}

func TestPacketResetKeepsParseCorrect(t *testing.T) {
	p := NewPacket()
	defer p.Close()
	for i := range p.bs {
		p.bs[i] = 0xa5
	}
	p.s = MpegTsPacketSize
	p.Offset = 42

	p.Reset()
	assert.Equal(t, uint(0), p.s)
	assert.Equal(t, int64(0), p.Offset)
	assert.Equal(t, PacketHeader{}, p.Header)
	assert.Nil(t, p.AdaptationField)
	assert.Nil(t, p.Payload)

	stream := offsetTestStream([]uint16{0x100})
	dmx := NewDemuxer(context.Background(), bytes.NewReader(stream), DemuxerOptPacketSize(MpegTsPacketSize))
	require.NoError(t, dmx.NextPacketTo(p))
	assert.Equal(t, uint16(0x100), p.Header.PID)
	assert.Equal(t, []byte{0xde, 0xad, 0xbe, 0xef}, p.Payload[:4])
}
