package tsio

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// growingRS is a seekable source that gains bytes after it reported EOF.
type growingRS struct {
	buf *bytes.Reader
}

// A source error is handed over once and the source is retried next time, as
// bufio does: a file that grows past its old end is followed.
func TestSeekBufferRetriesSourceAfterEOF(t *testing.T) {
	src := &growingRS{buf: bytes.NewReader(seq(6))}
	sb := NewSeekBuffer(64)
	require.NoError(t, sb.Reset(src))

	bs, err := sb.Peek(10)
	require.ErrorIs(t, err, io.EOF)
	assert.Equal(t, seq(6), bs)

	src.buf = bytes.NewReader(seq(10)[6:])
	bs, err = sb.Peek(10)
	require.NoError(t, err)
	assert.Equal(t, seq(10), bs)
}

func (g *growingRS) Read(p []byte) (int, error) { return g.buf.Read(p) }

func (g *growingRS) Seek(offset int64, whence int) (int64, error) {
	return g.buf.Seek(offset, whence)
}
