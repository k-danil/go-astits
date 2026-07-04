package demux

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v2/ts"
)

func TestZeroCopyPackets(t *testing.T) {
	const keepPID, skipPID = 0x100, 0x101
	pids := []uint16{keepPID, skipPID, skipPID, keepPID, skipPID, keepPID}
	stream := offsetTestStream(pids)

	tests := []struct {
		name        string
		batch       uint
		skipper     ts.PacketSkipper
		trailing    []byte
		wantOffsets []int64
	}{
		{"batchSmallerThanStream", 4, nil, nil, []int64{0, 188, 376, 564, 752, 940}},
		{"batchLargerThanStream", 64, nil, nil, []int64{0, 188, 376, 564, 752, 940}},
		{"singlePacketBatch", 1, nil, nil, []int64{0, 188, 376, 564, 752, 940}},
		{
			"skipperAdvancesOffset", 4,
			func(p *ts.Packet) bool { return p.Header.PID == skipPID },
			nil,
			[]int64{0, 564, 940},
		},
		{"trailingGarbageDropped", 4, nil, bytes.Repeat([]byte{0xff}, 50), []int64{0, 188, 376, 564, 752, 940}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := []func(*Demuxer){
				WithPacketSize(ts.PacketSize),
				WithZeroCopyPackets(tc.batch),
			}
			if tc.skipper != nil {
				opts = append(opts, WithPacketSkipper(tc.skipper))
			}
			dmx := New(context.Background(), bytes.NewReader(append(stream, tc.trailing...)), opts...)

			p := ts.NewPacket()
			defer p.Close()
			var offsets []int64
			for {
				if err := dmx.NextPacketTo(p); err != nil {
					require.True(t, errors.Is(err, ts.ErrNoMorePackets))
					break
				}
				assert.Equal(t, []byte{0xde, 0xad, 0xbe, 0xef}, p.Payload[:4])
				offsets = append(offsets, p.Offset)
			}
			assert.Equal(t, tc.wantOffsets, offsets)
		})
	}
}

func TestZeroCopyNextDataForbidden(t *testing.T) {
	stream := offsetTestStream([]uint16{0x100})
	dmx := New(context.Background(), bytes.NewReader(stream),
		WithPacketSize(ts.PacketSize),
		WithZeroCopyPackets(4),
	)
	_, err := dmx.NextData()
	assert.True(t, errors.Is(err, ErrZeroCopyNextData))
}

func TestZeroCopyViewLifetime(t *testing.T) {
	const batch = 2
	stream := offsetTestStream([]uint16{0x100, 0x101, 0x102, 0x103})
	dmx := New(context.Background(), bytes.NewReader(stream),
		WithPacketSize(ts.PacketSize),
		WithZeroCopyPackets(batch),
	)

	p := ts.NewPacket()
	defer p.Close()

	require.NoError(t, dmx.NextPacketTo(p))
	firstView := p.Payload
	require.NoError(t, dmx.NextPacketTo(p))
	sameBatchView := p.Payload

	// Within one batch, views of neighboring packets are alive simultaneously
	assert.Equal(t, []byte{0xde, 0xad, 0xbe, 0xef}, firstView[:4])
	assert.Equal(t, []byte{0xde, 0xad, 0xbe, 0xef}, sameBatchView[:4])

	// This call refills the batch — previous views die by contract
	require.NoError(t, dmx.NextPacketTo(p))
	assert.Equal(t, uint16(0x102), p.Header.PID)
}
