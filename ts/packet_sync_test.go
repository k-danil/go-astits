package ts

import (
	"bufio"
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// syncPacket builds one minimal valid 188-byte TS packet (sync, has-payload,
// zero payload) — enough for p.parse to accept it.
func syncPacket() []byte {
	p := make([]byte, PacketSize)
	p[0] = syncByte
	p[3] = 0x10 // payload present, no adaptation field
	return p
}

func syncPackets(n int) []byte {
	var b []byte
	for range n {
		b = append(b, syncPacket()...)
	}
	return b
}

// drainSync reads every packet a sync-locked buffer yields, returning their
// byte offsets and the terminating error.
func drainSync(t *testing.T, r *bytes.Reader, cfg PacketBufferConfig) (offsets []int64, err error) {
	t.Helper()
	pb, nerr := NewPacketBuffer(r, cfg)
	require.NoError(t, nerr)
	p := NewPacket()
	for {
		if err = pb.Next(p); err != nil {
			return
		}
		offsets = append(offsets, p.Offset)
	}
}

func TestSyncLockStartOffset(t *testing.T) {
	const junk = 12 // RTP-header-sized prefix before the first sync byte
	stream := append(make([]byte, junk), syncPackets(5)...)

	offsets, err := drainSync(t, bytes.NewReader(stream), PacketBufferConfig{SyncLock: true})
	require.ErrorIs(t, err, ErrNoMorePackets)
	assert.Equal(t, []int64{junk, junk + 188, junk + 376, junk + 564, junk + 752}, offsets,
		"offsets are true stream positions, first past the junk prefix")
}

func TestSyncLockMidStreamResync(t *testing.T) {
	const torn = 100 // not a multiple of 188 → breaks alignment
	var stream []byte
	stream = append(stream, syncPackets(3)...)
	stream = append(stream, make([]byte, torn)...)
	stream = append(stream, syncPackets(3)...)

	offsets, err := drainSync(t, bytes.NewReader(stream), PacketBufferConfig{SyncLock: true})
	require.ErrorIs(t, err, ErrNoMorePackets)
	require.Len(t, offsets, 6, "all six packets survive the torn gap")
	// Last packet before the gap, then first after it: the jump is one packet
	// plus the torn bytes, and the offset map stays true to stream position.
	assert.Equal(t, int64(2*188), offsets[2])
	assert.Equal(t, int64(3*188+torn), offsets[3])
}

func TestSyncLockZeroCopy(t *testing.T) {
	const junk = 7
	stream := append(make([]byte, junk), syncPackets(5)...)

	offsets, err := drainSync(t, bytes.NewReader(stream), PacketBufferConfig{SyncLock: true, ZeroCopyBatch: 4})
	require.ErrorIs(t, err, ErrNoMorePackets)
	assert.Equal(t, []int64{junk, junk + 188, junk + 376, junk + 564, junk + 752}, offsets)
}

func TestSyncLockFixedSize(t *testing.T) {
	const junk = 9
	stream := append(make([]byte, junk), syncPackets(4)...)

	offsets, err := drainSync(t, bytes.NewReader(stream), PacketBufferConfig{SyncLock: true, PacketSize: PacketSize})
	require.ErrorIs(t, err, ErrNoMorePackets)
	assert.Len(t, offsets, 4)
	assert.Equal(t, int64(junk), offsets[0])
}

func TestSyncLockResyncLimit(t *testing.T) {
	var stream []byte
	stream = append(stream, syncPackets(3)...)
	stream = append(stream, make([]byte, 4000)...) // junk to EOF, no re-lock possible

	// Bounded: gives up with a matchable error after the fruitless windows.
	_, err := drainSync(t, bytes.NewReader(stream), PacketBufferConfig{SyncLock: true, ResyncLimit: 2})
	require.ErrorIs(t, err, ErrInvalidData)

	// Unbounded (default): scans to EOF and ends cleanly.
	offsets, err := drainSync(t, bytes.NewReader(stream), PacketBufferConfig{SyncLock: true})
	require.ErrorIs(t, err, ErrNoMorePackets)
	assert.Len(t, offsets, 3)
}

func TestSyncLockOffThenOffsetFails(t *testing.T) {
	const junk = 12
	stream := append(make([]byte, junk), syncPackets(5)...)

	// Without sync lock an offset stream must not silently pass detection.
	_, err := NewPacketBuffer(bytes.NewReader(stream), PacketBufferConfig{})
	require.ErrorIs(t, err, ErrPacketMustStartWithASyncByte)
}

// corruptPacket has a valid sync byte but an adaptation-field length that
// overruns the packet, so it aligns yet fails to parse.
func corruptPacket() []byte {
	p := make([]byte, PacketSize)
	p[0] = syncByte
	p[3] = 0x20 // adaptation field present, no payload
	p[4] = 200  // AF length > packet body → ErrShortPacket
	return p
}

func TestSyncLockSkipsCorruptPacket(t *testing.T) {
	var stream []byte
	stream = append(stream, syncPackets(3)...)
	stream = append(stream, corruptPacket()...)
	stream = append(stream, syncPackets(3)...)

	// Default (unbounded): the corrupt aligned packet is dropped, not fatal.
	offsets, err := drainSync(t, bytes.NewReader(stream), PacketBufferConfig{SyncLock: true})
	require.ErrorIs(t, err, ErrNoMorePackets)
	require.Len(t, offsets, 6)
	assert.Equal(t, int64(2*188), offsets[2])
	assert.Equal(t, int64(4*188), offsets[3], "offset jumps across the dropped packet")

	// Bounded: one damage event at the limit gives up with a matchable error.
	_, err = drainSync(t, bytes.NewReader(stream), PacketBufferConfig{SyncLock: true, ResyncLimit: 1})
	require.ErrorIs(t, err, ErrInvalidData)
}

type nonSeekableReader struct{ r io.Reader }

func (n nonSeekableReader) Read(p []byte) (int, error) { return n.r.Read(p) }

func TestAutoDetectNonSeekableNoOffsetSkew(t *testing.T) {
	stream := syncPackets(5)
	pb, err := NewPacketBuffer(nonSeekableReader{bytes.NewReader(stream)}, PacketBufferConfig{})
	require.NoError(t, err)
	require.Equal(t, uint(PacketSize), pb.PacketSize())

	p := NewPacket()
	var offsets []int64
	for {
		if perr := pb.Next(p); perr != nil {
			require.ErrorIs(t, perr, ErrNoMorePackets)
			break
		}
		offsets = append(offsets, p.Offset)
	}
	// A non-seekable reader is buffered internally, so autodetect neither drops
	// packets nor skews the offset map.
	assert.Equal(t, []int64{0, 188, 376, 564, 752}, offsets)
}

func FuzzSyncLock(f *testing.F) {
	f.Add(syncPackets(3))
	f.Add(append(make([]byte, 12), syncPackets(3)...))
	f.Add(append(syncPackets(2), make([]byte, 500)...))
	f.Fuzz(func(t *testing.T, b []byte) {
		pb, err := NewPacketBuffer(bytes.NewReader(b), PacketBufferConfig{SyncLock: true, ResyncLimit: 4})
		if err != nil {
			return
		}
		p := NewPacket()
		for range 10000 {
			if pb.Next(p) != nil {
				return
			}
		}
	})
}

func TestAsPeeker(t *testing.T) {
	br := bufio.NewReader(bytes.NewReader(nil))
	assert.Same(t, br, asPeeker(br, 1024), "an existing *bufio.Reader is used directly")

	_, ok := asPeeker(bytes.NewReader(nil), 1024).(*bufio.Reader)
	assert.True(t, ok, "a plain reader is wrapped in bufio")
}
