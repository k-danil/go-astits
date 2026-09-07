package demux

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v3/ts"
)

func rawPacket(cc byte) (p []byte) {
	p = make([]byte, ts.PacketSize)
	p[0], p[1], p[2], p[3] = 0x47, 0x01, 0x00, 0x10|cc
	for i := 4; i < len(p); i++ {
		p[i] = 0xAA
	}
	return
}

func rawPackets(first, last byte) (raw []byte) {
	for cc := first; cc <= last; cc++ {
		raw = append(raw, rawPacket(cc)...)
	}
	return
}

func holedDemuxer(raw []byte, hook func(*ts.Packet)) *Demuxer {
	return New(context.Background(), bytes.NewReader(raw),
		WithPacketSize(ts.PacketSize), WithSyncLock(), WithSkipErrLimit(16), WithResyncLimit(-1), WithRecoverableErrors(),
		WithPacketHook(hook))
}

func TestBufferErrorPrecedesThePacketAfterTheLoss(t *testing.T) {
	junk := make([]byte, 300)
	tests := []struct {
		name  string
		raw   []byte
		after byte
	}{
		{name: "a hole mid-stream", raw: slices.Concat(rawPackets(0, 7), junk, rawPackets(8, 15)), after: 8},
		{name: "junk ahead of the first packet", raw: slices.Concat(junk, rawPackets(0, 15)), after: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var order []string
			dmx := holedDemuxer(tt.raw, func(p *ts.Packet) {
				if p.Header.ContinuityCounter == tt.after {
					order = append(order, "packet after the loss")
				}
			})
			defer dmx.Close()
			for _, err := range dmx.Events() {
				if re, ok := errors.AsType[*ts.RecoverableError](err); ok && re.Kind == ts.ErrorKindSyncLoss {
					order = append(order, "event")
					assert.EqualValues(t, len(junk), re.Dropped)
				}
			}
			assert.Equal(t, []string{"event", "packet after the loss"}, order)
		})
	}
}

func TestHeldPacketIsDroppedByRewind(t *testing.T) {
	var seen []byte
	dmx := holedDemuxer(slices.Concat(rawPackets(0, 7), make([]byte, 300), rawPackets(8, 15)),
		func(p *ts.Packet) { seen = append(seen, p.Header.ContinuityCounter) })
	defer dmx.Close()
	for {
		_, err := dmx.Next()
		if re, ok := errors.AsType[*ts.RecoverableError](err); ok && re.Kind == ts.ErrorKindSyncLoss {
			break
		}
		require.NoError(t, err)
	}
	_, err := dmx.Rewind()
	require.NoError(t, err)
	seen = seen[:0]
	_, _ = dmx.Next()
	require.NotEmpty(t, seen)
	assert.EqualValues(t, 0, seen[0])
}

func TestHeldPacketStillYieldsItsUnit(t *testing.T) {
	dmx := holedDemuxer(slices.Concat(rawPackets(0, 7), make([]byte, 300), validPATPacket(), rawPackets(8, 15)), nil)
	defer dmx.Close()
	var pat bool
	for ev, err := range dmx.Events() {
		pat = pat || err == nil && ev == EventPAT
	}
	assert.True(t, pat)
}
