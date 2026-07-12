package ts

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncLockReportsRecoverable(t *testing.T) {
	const torn = 100 // not a multiple of 188 → breaks alignment

	tornStream := func() (s []byte) {
		s = append(s, syncPackets(3)...)
		s = append(s, make([]byte, torn)...)
		s = append(s, syncPackets(3)...)
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
			cfg := PacketBufferConfig{SyncLock: true, OnRecover: func(e RecoverableError) {
				got = append(got, e)
			}}
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

func TestNoRecoverWithoutHook(t *testing.T) {
	var stream []byte
	stream = append(stream, syncPackets(3)...)
	stream = append(stream, corruptPacket()...)
	stream = append(stream, syncPackets(3)...)

	offsets, err := drainSync(t, bytes.NewReader(stream), PacketBufferConfig{SyncLock: true})
	require.ErrorIs(t, err, ErrNoMorePackets)
	assert.Len(t, offsets, 6, "recovery still happens with a nil hook")
}
