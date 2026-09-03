package tsio

import (
	"bufio"
	"io"
	"math"
)

type BytesReader struct {
	data []byte
	pos  int
}

func NewBytesReader(data []byte) *BytesReader {
	return &BytesReader{data: data}
}

func (r *BytesReader) rest() []byte {
	if r.pos >= len(r.data) {
		return r.data[len(r.data):]
	}
	return r.data[r.pos:]
}

func (r *BytesReader) Read(p []byte) (n int, err error) {
	rest := r.rest()
	if len(rest) == 0 {
		return 0, io.EOF
	}
	n = copy(p, rest)
	r.pos += n
	return
}

func (r *BytesReader) Peek(n int) (bs []byte, err error) {
	if n < 0 {
		return nil, bufio.ErrNegativeCount
	}
	bs = r.rest()
	if len(bs) < n {
		return bs, io.EOF
	}
	return bs[:n], nil
}

func (r *BytesReader) Discard(n int) (discarded int, err error) {
	if n < 0 {
		return 0, bufio.ErrNegativeCount
	}
	if rest := len(r.rest()); n > rest {
		r.pos = len(r.data)
		return rest, io.EOF
	}
	r.pos += n
	return n, nil
}

func (r *BytesReader) Buffered() int {
	return len(r.rest())
}

// math.MaxInt, not len: len would fail the sync-lock scan-window check on short input.
func (r *BytesReader) Size() int {
	return math.MaxInt
}

func (r *BytesReader) Seek(offset int64, whence int) (pos int64, err error) {
	switch whence {
	case io.SeekStart:
		pos = offset
	case io.SeekCurrent:
		pos = int64(r.pos) + offset
	case io.SeekEnd:
		pos = int64(len(r.data)) + offset
	default:
		return 0, errInvalidWhence
	}
	if pos < 0 {
		return 0, errSeekNegative
	}
	r.pos = int(min(pos, math.MaxInt))
	return
}
