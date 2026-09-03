// Package tsio holds the reader contract the packet buffer reads through, plus BytesReader (input already in memory or mapped) and SeekBuffer (a seekable source through a bounded buffer).
package tsio

// Peeker is what the packet buffer reads through; *bufio.Reader satisfies it.
//
// Contract, as *bufio.Reader: Peek returns up to n bytes without consuming (fewer, with a non-nil error, only at end of input) and must accept n up to Size(); Discard drops exactly n, never more than a preceding Peek returned; Size is that ceiling and must be at least 1024 — a smaller one fails ts.NewPacketBuffer under SyncLock; a source without a ceiling reports math.MaxInt.
type Peeker interface {
	Peek(n int) ([]byte, error)
	Discard(n int) (discarded int, err error)
	Size() int
	// Bytes Peek can return without reading the source; windows are cut to it, so a live feed never waits for more than one packet.
	Buffered() int
}

// Tag is opaque (a receive timestamp, an RTP sequence, a frame index) and belongs to the first byte the last Peek returned. Buffered must stop at a tag boundary, so a window never spans two tags. Size must be at least 204, or the Peeker is wrapped in bufio and its tags are lost; under sync lock, below 1024 fails NewPacketBuffer outright.
type Tagger interface {
	Peeker
	Tag() uint64
}
