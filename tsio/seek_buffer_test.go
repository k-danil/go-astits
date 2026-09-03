package tsio

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingRS counts source-level Read/Seek so tests can prove an in-buffer seek
// touches neither.
type countingRS struct {
	rs    io.ReadSeeker
	reads int
	seeks int
}

func (c *countingRS) Read(p []byte) (int, error) {
	c.reads++
	return c.rs.Read(p)
}

func (c *countingRS) Seek(offset int64, whence int) (int64, error) {
	c.seeks++
	return c.rs.Seek(offset, whence)
}

func seq(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

func newBuf(t *testing.T, data []byte, size int) (*SeekBuffer, *countingRS) {
	t.Helper()
	src := &countingRS{rs: bytes.NewReader(data)}
	sb := NewSeekBuffer(size)
	require.NoError(t, sb.Reset(src))
	return sb, src
}

var (
	_ io.ReadSeeker = (*SeekBuffer)(nil)
	_ Peeker        = (*SeekBuffer)(nil)
	_ io.ReadSeeker = (*BytesReader)(nil)
	_ Peeker        = (*BytesReader)(nil)
)

func TestSeekBufferPeekBufferFull(t *testing.T) {
	data := seq(1000)
	sb, _ := newBuf(t, data, 256)

	bs, err := sb.Peek(512)
	require.ErrorIs(t, err, bufio.ErrBufferFull)
	assert.Equal(t, data[:256], bs)
}

func TestSeekBufferSeekOutsideBufferReseeksSource(t *testing.T) {
	data := seq(5000)
	sb, src := newBuf(t, data, 512)

	// Advance far enough that byte 0 has been evicted by slides.
	_, err := sb.Discard(2000)
	require.NoError(t, err)
	seeksBefore := src.seeks

	pos, err := sb.Seek(0, io.SeekStart)
	require.NoError(t, err)
	assert.Equal(t, int64(0), pos)
	assert.Equal(t, seeksBefore+1, src.seeks, "evicted target must reseek source")

	got := make([]byte, 100)
	_, err = io.ReadFull(sb, got)
	require.NoError(t, err)
	assert.Equal(t, data[:100], got)
}

func TestSeekBufferSeekCurrent(t *testing.T) {
	data := seq(3000)
	sb, _ := newBuf(t, data, 4096)

	_, err := sb.Discard(700)
	require.NoError(t, err)

	pos, err := sb.Seek(0, io.SeekCurrent)
	require.NoError(t, err)
	assert.Equal(t, int64(700), pos)

	pos, err = sb.Seek(-200, io.SeekCurrent)
	require.NoError(t, err)
	assert.Equal(t, int64(500), pos)

	got := make([]byte, 50)
	_, err = io.ReadFull(sb, got)
	require.NoError(t, err)
	assert.Equal(t, data[500:550], got)
}

// TestSeekBufferPeekDiscardWindow mirrors the astits peek-batch loop: peek a
// full window, consume a floored part via Discard, peek the next window — the
// stream must stay continuous across the boundary.
func TestSeekBufferPeekDiscardWindow(t *testing.T) {
	const size = 512
	data := seq(4000)
	sb, _ := newBuf(t, data, size)

	var got []byte
	for {
		bs, err := sb.Peek(size)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
			require.NoError(t, err)
		}
		if len(bs) == 0 {
			break
		}
		got = append(got, bs...)
		if _, err = sb.Discard(len(bs)); err != nil {
			require.ErrorIs(t, err, io.EOF)
		}
	}
	assert.Equal(t, data, got)
}

func TestSeekBufferResetAnchorsAtCurrentOffset(t *testing.T) {
	data := seq(2000)
	src := bytes.NewReader(data)
	_, err := src.Seek(600, io.SeekStart)
	require.NoError(t, err)

	sb := NewSeekBuffer(4096)
	require.NoError(t, sb.Reset(src))

	pos, err := sb.Seek(0, io.SeekCurrent)
	require.NoError(t, err)
	assert.Equal(t, int64(600), pos)

	got := make([]byte, 100)
	_, err = io.ReadFull(sb, got)
	require.NoError(t, err)
	assert.Equal(t, data[600:700], got)
}

// The packet buffer probes for more input with a one-packet Peek once the
// buffer is drained; that probe must not evict the window, or a rewind after a
// full pass would re-read the source.
func TestSeekBufferSeekBackAfterEOFNoSourceIO(t *testing.T) {
	data := seq(3000)
	sb, src := newBuf(t, data, 4096)

	_, err := sb.Discard(len(data))
	require.NoError(t, err)
	_, err = sb.Peek(188)
	require.ErrorIs(t, err, io.EOF)
	readsAtEnd, seeksAtEnd := src.reads, src.seeks

	_, err = sb.Seek(0, io.SeekStart)
	require.NoError(t, err)
	assert.Equal(t, readsAtEnd, src.reads)
	assert.Equal(t, seeksAtEnd, src.seeks)

	got, err := io.ReadAll(sb)
	require.NoError(t, err)
	assert.Equal(t, data, got)
}
