package ts

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			r := bytes.NewReader(tc.bs)
			size, err := autoDetectPacketSize(r)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.size, size)
			assert.Equal(t, len(tc.bs), r.Len(), "seekable reader rewound to start")
		})
	}
}
