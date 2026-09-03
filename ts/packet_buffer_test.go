package ts

import (
	"context"
	"io"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v3/tsio"
)

// syncStream builds total bytes with a sync byte at start, start+size, … for
// syncs occurrences; every other byte is 0x00 (not a sync byte).
func syncStream(start, size, syncs, total int) []byte {
	bs := make([]byte, total)
	for i, off := 0, start; i < syncs && off < total; i, off = i+1, off+size {
		bs[off] = syncByte
	}
	return bs
}

func TestAutoDetectPacketSize(t *testing.T) {
	// A 204 stream whose Reed-Solomon parity happens to hold a 0x47 at offset
	// 188 must still lock onto 204, not be fooled into an aligned-188 read.
	rs204Spurious := syncStream(0, RSPacketSize, autoDetectSyncs, 3*RSPacketSize)
	rs204Spurious[PacketSize] = syncByte

	singleSync := make([]byte, 3*PacketSize)
	singleSync[0] = syncByte

	for _, tc := range []struct {
		name string
		bs   []byte
		size uint
		err  error
	}{
		{name: "no sync at boundary", bs: []byte{0x02, syncByte}, err: ErrPacketMustStartWithASyncByte},
		{name: "sync but no periodic lock", bs: singleSync, err: ErrInvalidData},
		{name: "188 TS", bs: syncStream(0, PacketSize, autoDetectSyncs, 3*PacketSize), size: PacketSize},
		{name: "204 Reed-Solomon", bs: syncStream(0, RSPacketSize, autoDetectSyncs, 3*RSPacketSize), size: RSPacketSize},
		{name: "192 M2TS", bs: syncStream(M2TSPacketSize-PacketSize, M2TSPacketSize, autoDetectSyncs, 3*M2TSPacketSize), size: M2TSPacketSize},
		{name: "204 not fooled by 0x47 at 188", bs: rs204Spurious, size: RSPacketSize},
	} {
		t.Run(tc.name, func(t *testing.T) {
			size, err := autoDetectPacketSize(tsio.NewBytesReader(tc.bs))
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.size, size)
		})
	}
}

// EOF past avail is not the end: the source keeps growing.
type growingReader struct {
	data  []byte
	avail int
	pos   int
}

func (r *growingReader) Read(p []byte) (n int, err error) {
	if r.pos >= r.avail {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:r.avail])
	r.pos += n
	return
}

func TestGrowingStreamKeepsShortTail(t *testing.T) {
	const tail = 100
	for _, mode := range []struct {
		name string
		cfg  PacketBufferConfig
	}{
		{"plain", PacketBufferConfig{PacketSize: PacketSize}},
		{"sync lock", PacketBufferConfig{SyncLock: true}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			stream := syncPackets(6)
			r := &growingReader{data: stream, avail: 3*PacketSize + tail}

			var got []RecoverableError
			cfg := mode.cfg
			cfg.OnRecover = func(e RecoverableError) { got = append(got, e) }
			pb, err := NewPacketBuffer(context.Background(), r, cfg)
			require.NoError(t, err)
			p := NewPacket()
			defer p.Close()

			drain := func() (offsets []int64) {
				for {
					if err = pb.Next(p); err != nil {
						return
					}
					offsets = append(offsets, p.Offset)
				}
			}

			assert.Equal(t, []int64{0, PacketSize, 2 * PacketSize}, drain())
			require.ErrorIs(t, err, ErrNoMorePackets)
			require.Len(t, got, 1)
			assert.Equal(t, ErrorKindPacketDrop, got[0].Kind)
			assert.Equal(t, int64(3*PacketSize), got[0].Offset)
			assert.Equal(t, int64(tail), got[0].Dropped)

			require.ErrorIs(t, pb.Next(p), ErrNoMorePackets)
			assert.Len(t, got, 1, "the same stall is reported once")

			r.avail = len(stream)
			assert.Equal(t, []int64{3 * PacketSize, 4 * PacketSize, 5 * PacketSize}, drain(),
				"the held tail completes the packet it began")
			require.ErrorIs(t, err, ErrNoMorePackets)
			assert.Equal(t, int64(len(stream)), pb.Pos())
		})
	}
}

func TestAdvanceRejectsOutOfRange(t *testing.T) {
	pb, err := NewPacketBuffer(context.Background(), tsio.NewBytesReader(syncPackets(3)), PacketBufferConfig{PacketSize: PacketSize, ZeroCopyBatch: 4})
	require.NoError(t, err)
	w, err := pb.Window()
	require.NoError(t, err)
	require.Len(t, w, 3*PacketSize)

	for _, n := range []int{1, -1, len(w) + PacketSize} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			require.ErrorIs(t, pb.Advance(n), ErrInvalidData)
			assert.Zero(t, pb.Pos(), "a rejected advance consumes nothing")
		})
	}

	require.NoError(t, pb.Advance(len(w)))
	assert.Equal(t, int64(len(w)), pb.Pos())
}
