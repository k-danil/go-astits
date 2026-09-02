package ts

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncLockReportsRecoverable(t *testing.T) {
	const torn = 100 // not a multiple of 188 → breaks alignment

	tornStream := func() (s []byte) {
		s = append(s, syncPackets(3)...)
		s = append(s, make([]byte, torn)...)
		s = append(s, syncPackets(5)...)
		return
	}()
	corruptStream := func() (s []byte) {
		s = append(s, syncPackets(3)...)
		s = append(s, corruptPacket()...)
		s = append(s, syncPackets(3)...)
		return
	}()

	tests := []struct {
		name   string
		stream []byte
		want   ErrorKind
	}{
		{"sync loss on torn gap", tornStream, ErrorKindSyncLoss},
		{"packet drop on corrupt aligned packet", corruptStream, ErrorKindPacketDrop},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []RecoverableError
			cfg := lossy
			cfg.OnRecover = func(e RecoverableError) {
				got = append(got, e)
			}
			_, err := drainSync(t, bytes.NewReader(tt.stream), cfg)
			require.ErrorIs(t, err, ErrNoMorePackets)
			require.NotEmpty(t, got)

			var seen bool
			for _, e := range got {
				assert.Equal(t, PIDUnset, e.PID, "sync-level event is not bound to a PID")
				require.Error(t, e.Err)
				assert.ErrorIs(t, e.Err, ErrInvalidData)
				if e.Kind == tt.want {
					seen = true
				}
			}
			assert.True(t, seen, "expected a %s event", tt.want)
		})
	}
}

// One event per loss with the byte count summed across scan windows, the EOF
// tail included; asking for more after EOF reports nothing new.
func TestSyncLossDroppedSpansScanWindows(t *testing.T) {
	tests := []struct {
		name     string
		torn     int
		trailing bool // the junk runs to EOF
		want     []ErrorKind
	}{
		{"one window", 100, false, []ErrorKind{ErrorKindSyncLoss}},
		{"several windows", syncScanWindow + 84, false, []ErrorKind{ErrorKindSyncLoss}},
		{"junk to EOF longer than a packet", 400, true, []ErrorKind{ErrorKindSyncLoss}},
		{"junk to EOF shorter than a packet", 100, true, []ErrorKind{ErrorKindPacketDrop}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stream []byte
			stream = append(stream, syncPackets(3)...)
			stream = append(stream, make([]byte, tt.torn)...)
			wantPackets := 3
			if !tt.trailing {
				stream = append(stream, syncPackets(5)...)
				wantPackets = 8
			}

			var got []RecoverableError
			cfg := lossy
			cfg.OnRecover = func(e RecoverableError) {
				got = append(got, e)
			}
			pb, err := NewPacketBuffer(context.Background(), bytes.NewReader(stream), cfg)
			require.NoError(t, err)
			p := NewPacket()
			var packets int
			for ; err == nil; err = pb.Next(p) {
				packets++
			}
			require.ErrorIs(t, err, ErrNoMorePackets)
			assert.Equal(t, wantPackets, packets-1)

			var kinds []ErrorKind
			var dropped int64
			for _, e := range got {
				kinds = append(kinds, e.Kind)
				dropped += e.Dropped
			}
			assert.Equal(t, tt.want, kinds)
			assert.Equal(t, int64(tt.torn), dropped)
			require.NotEmpty(t, got)
			assert.Equal(t, int64(3*PacketSize), got[0].Offset)

			require.ErrorIs(t, pb.Next(p), ErrNoMorePackets)
			assert.Len(t, got, len(tt.want), "EOF asked again reports nothing new")
		})
	}
}

func TestNoRecoverWithoutHook(t *testing.T) {
	var stream []byte
	stream = append(stream, syncPackets(3)...)
	stream = append(stream, corruptPacket()...)
	stream = append(stream, syncPackets(3)...)

	offsets, err := drainSync(t, bytes.NewReader(stream), lossy)
	require.ErrorIs(t, err, ErrNoMorePackets)
	assert.Len(t, offsets, 6, "recovery still happens with a nil hook")
}
