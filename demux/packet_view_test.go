package demux

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
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

// The accumulator copies payloads out before the batch refill, so the event
// pump is legal in zero-copy mode.
func TestZeroCopyNext(t *testing.T) {
	stream := fuzzSeedStream()
	dmx := New(context.Background(), bytes.NewReader(stream),
		WithPacketSize(192),
		WithZeroCopyPackets(1), // refill on every packet: the strictest lifetime
	)
	events := 0
	for {
		if _, err := dmx.Next(); err != nil {
			assert.True(t, errors.Is(err, ts.ErrNoMorePackets))
			break
		}
		events++
	}
	assert.NotZero(t, events)
	assert.NotNil(t, dmx.PAT())
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
	require.Equal(t, len(copyRaws), len(viewRaws))
	for i := range copyRaws {
		require.Len(t, copyRaws[i], ts.PacketSize)
		assert.Equal(t, copyRaws[i], viewRaws[i], "packet %d", i)
	}
}

// A *bufio.Reader source routes view mode through the peek path — packets are
// views straight into bufio's buffer, no second copy into an owned batch. Its
// output must be byte-identical to copy mode.
func TestPeekViewMatchesCopy(t *testing.T) {
	stream := offsetTestStream([]uint16{0x100, 0x101, 0x102, 0x103, 0x104, 0x105})

	walk := func(r io.Reader, zeroCopy bool) (raws [][]byte) {
		opts := []func(*Demuxer){WithPacketSize(ts.PacketSize)}
		if zeroCopy {
			opts = append(opts, WithZeroCopyPackets(2))
		}
		dmx := New(context.Background(), r, opts...)
		p := ts.NewPacket()
		defer p.Close()
		for {
			if err := dmx.NextPacketTo(p); err != nil {
				return
			}
			raws = append(raws, bytes.Clone(p.Raw()))
		}
	}

	copyRaws := walk(bytes.NewReader(stream), false)
	peekRaws := walk(bufio.NewReaderSize(bytes.NewReader(stream), 4096), true)
	require.NotEmpty(t, copyRaws)
	require.Equal(t, len(copyRaws), len(peekRaws))
	for i := range copyRaws {
		require.Len(t, copyRaws[i], ts.PacketSize)
		assert.Equal(t, copyRaws[i], peekRaws[i], "packet %d", i)
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
