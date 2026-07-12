package demux

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v2/psi"
	"github.com/k-danil/go-astits/v2/ts"
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
	for range 3 {
		stream = append(stream, minimalTSPacket()...)
	}
	stream = append(stream, corruptAlignedPacket()...)

	dmx := New(context.Background(), bytes.NewReader(stream),
		WithSyncLock(), WithResyncLimit(1), WithRecoverableErrors())

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
	assert.ErrorIs(t, err, ts.ErrInvalidData)
	assert.NotErrorIs(t, err, ts.ErrNoMorePackets)
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
		assert.ErrorIs(t, err, psi.ErrCRC32Mismatch)
		assert.ErrorIs(t, err, ts.ErrInvalidData)
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
