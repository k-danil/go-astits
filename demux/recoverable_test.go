package demux

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v3/psi"
	"github.com/k-danil/go-astits/v3/ts"
)

func validPATPacket() []byte {
	return hexToBytes(`474000100000b00d0001c100000001f0002ab104b2ffffffffffffffff
		ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff
		ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff
		ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff
		ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff
		ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff
		ffffffffffffffffff`)
}

// patCRC32LSB is the offset of the CRC32 low byte within validPATPacket.
const patCRC32LSB = 20

// corruptCRCPATPacket flips a CRC32 bit so the section parses structurally but
// fails the checksum (TR 101 290 CRC_error).
func corruptCRCPATPacket() []byte {
	p := validPATPacket()
	p[patCRC32LSB] ^= 0x01
	return p
}

func minimalTSPacket() []byte {
	p := make([]byte, ts.PacketSize)
	p[0] = syncByte
	p[1] = 0x01 // PID 0x100, non-PSI
	p[3] = 0x10 // payload only
	return p
}

// corruptAlignedPacket has a valid sync byte but an adaptation-field length that
// overruns the packet, so it aligns yet fails to parse.
func corruptAlignedPacket() []byte {
	p := make([]byte, ts.PacketSize)
	p[0] = syncByte
	p[3] = 0x20 // adaptation field present, no payload
	p[4] = 200  // AF length > packet body
	return p
}

// A fatal read error must not swallow the recoverable errors queued during the
// same read: the damage event surfaces before the stream-ending fatal.
func TestDemuxerRecoverableFlushedBeforeFatal(t *testing.T) {
	var stream []byte
	for cc := range 3 {
		p := minimalTSPacket()
		p[3] |= byte(cc)
		stream = append(stream, p...)
	}
	stream = append(stream, corruptAlignedPacket()...)

	dmx := New(context.Background(), bytes.NewReader(stream),
		WithSyncLock(), WithRecoverableErrors())

	ev, err := dmx.Next()
	require.Error(t, err)
	assert.Equal(t, EventError, ev)
	require.True(t, ts.IsRecoverable(err), "damage event surfaces first")
	var re *ts.RecoverableError
	require.ErrorAs(t, err, &re)
	assert.Equal(t, ts.ErrorKindPacketDrop, re.Kind)

	_, err = dmx.Next()
	require.Error(t, err)
	assert.False(t, ts.IsRecoverable(err), "then the fatal ends the stream")
	require.ErrorIs(t, err, ts.ErrInvalidData)
	assert.NotErrorIs(t, err, ts.ErrNoMorePackets)
}

// A unit-level error is charged to the unit's last packet, not the packet under the reader.
func TestRecoverableOffsetIsOwnPacket(t *testing.T) {
	garbage := []byte{0x11, 0x22, 0x33, 0x44, 0x55}
	tests := []struct {
		name       string
		stream     []byte
		wantOffset int64
	}{
		{
			name: "drained behind another PID",
			stream: bytes.Join([][]byte{
				payloadPacket(0x100, 0, true, garbage),
				payloadPacket(0x200, 0, true, garbage),
				payloadPacket(0x200, 1, false, garbage),
			}, nil),
			wantOffset: 0,
		},
		{
			name: "closed by the start of the next unit",
			stream: bytes.Join([][]byte{
				payloadPacket(0x100, 0, true, garbage),
				payloadPacket(0x100, 1, false, garbage),
				payloadPacket(0x100, 2, true, garbage),
			}, nil),
			wantOffset: ts.PacketSize,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dmx := New(context.Background(), bytes.NewReader(tt.stream), WithPacketSize(ts.PacketSize), WithRecoverableErrors())
			defer dmx.Close()

			var first *ts.RecoverableError
			for _, err := range dmx.Events() {
				if re, ok := errors.AsType[*ts.RecoverableError](err); ok && re.PID == 0x100 && first == nil {
					first = re
				}
			}
			require.NotNil(t, first)
			assert.Equal(t, ts.ErrorKindUnknownUnit, first.Kind)
			assert.Equal(t, tt.wantOffset, first.Offset)
		})
	}
}

func TestRecoverableContinuityWithoutLoss(t *testing.T) {
	stream := psiPacket(ts.PIDPAT, 0, patSection(0, true, 0x100))
	stream = append(stream, psiPacket(ts.PIDPAT, 5, patSection(1, true, 0x200))...)
	stream = append(stream, psiPacket(ts.PIDPAT, 6, patSection(2, true, 0x300))...)

	dmx := New(context.Background(), bytes.NewReader(stream), WithPacketSize(ts.PacketSize), WithRecoverableErrors())
	defer dmx.Close()

	var got []*ts.RecoverableError
	var tables int
	for ev, err := range dmx.Events() {
		if re, ok := errors.AsType[*ts.RecoverableError](err); ok {
			got = append(got, re)
			continue
		}
		require.NoError(t, err)
		if ev == EventPAT {
			tables++
		}
	}
	assert.Equal(t, 3, tables, "every table still arrives")
	require.Len(t, got, 1, "one event for the gap, none for the first packet or the packet after it")
	assert.Equal(t, ts.ErrorKindContinuity, got[0].Kind)
	assert.Equal(t, int64(ts.PacketSize), got[0].Offset)
	assert.Equal(t, int64(0), got[0].Dropped)
	assert.ErrorIs(t, got[0].Err, ts.ErrContinuityGap)
}

func TestDemuxerRecoverableCRCMismatch(t *testing.T) {
	t.Run("silent by default", func(t *testing.T) {
		dmx := New(context.Background(), bytes.NewReader(corruptCRCPATPacket()), WithPacketSize(188))
		_, err := dmx.Next()
		require.ErrorIs(t, err, ts.ErrNoMorePackets)
		assert.Nil(t, dmx.PAT(), "corrupt table dropped, none applied")
	})

	t.Run("surfaced under WithRecoverableErrors", func(t *testing.T) {
		dmx := New(context.Background(), bytes.NewReader(corruptCRCPATPacket()),
			WithPacketSize(188), WithRecoverableErrors())

		ev, err := dmx.Next()
		require.Error(t, err)
		assert.Equal(t, EventError, ev)

		var re *ts.RecoverableError
		require.ErrorAs(t, err, &re)
		assert.Equal(t, ts.ErrorKindCRC, re.Kind)
		assert.Equal(t, uint16(0), re.PID, "CRC error bound to the PAT PID")
		require.ErrorIs(t, err, psi.ErrCRC32Mismatch)
		require.ErrorIs(t, err, ts.ErrInvalidData)
		assert.True(t, ts.IsRecoverable(err))
		assert.Nil(t, dmx.PAT(), "corrupt table not applied")

		_, err = dmx.Next()
		require.ErrorIs(t, err, ts.ErrNoMorePackets, "recoverable error is non-terminal")
	})

	t.Run("valid PAT emits no spurious error", func(t *testing.T) {
		dmx := New(context.Background(), bytes.NewReader(validPATPacket()),
			WithPacketSize(188), WithRecoverableErrors())

		var pats int
		for ev, err := range dmx.Events() {
			require.NoError(t, err)
			if ev == EventPAT {
				pats++
			}
		}
		assert.Equal(t, 1, pats)
	})
}
