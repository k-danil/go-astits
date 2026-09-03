package tsio

import (
	"bufio"
	"errors"
	"io"
)

const maxEmptyReads = 100

const minSeekBufferSize = 16

var (
	errInvalidWhence = errors.New("astits: seek: invalid whence")
	errSeekNegative  = errors.New("astits: seek: negative position")
)

// SeekBuffer is a buffered reader over an io.ReadSeeker that also satisfies Peeker: a Seek landing inside the buffered window moves only the cursor, so a Rewind after a short prefix scan costs no I/O.
//
// Invariant: the source sits at bufStart+w and advances only through Read/fill, never through an in-buffer Seek; buf[r:w] is unconsumed, and a Peek/Read slice is valid only until the next fill.
type SeekBuffer struct {
	rs       io.ReadSeeker
	buf      []byte
	bufStart int64
	r, w     int
	err      error
}

// size caps the peek window and the seek-back reach. Attach a source with Reset before use.
func NewSeekBuffer(size int) *SeekBuffer {
	return &SeekBuffer{buf: make([]byte, max(size, minSeekBufferSize))}
}

// Anchors at rs's current offset, so Seek stays in rs's coordinate space.
func (s *SeekBuffer) Reset(rs io.ReadSeeker) (err error) {
	var pos int64
	if pos, err = rs.Seek(0, io.SeekCurrent); err != nil {
		return
	}
	s.rs = rs
	s.bufStart = pos
	s.r, s.w = 0, 0
	s.err = nil
	return
}

func (s *SeekBuffer) Size() int {
	return len(s.buf)
}

func (s *SeekBuffer) Buffered() int {
	return s.w - s.r
}

func (s *SeekBuffer) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return
	}
	if s.r == s.w {
		s.fill()
		if s.r == s.w {
			err, s.err = s.err, nil
			return
		}
	}
	n = copy(p, s.buf[s.r:s.w])
	s.r += n
	return
}

func (s *SeekBuffer) Peek(n int) (bs []byte, err error) {
	if n < 0 {
		err = bufio.ErrNegativeCount
		return
	}
	if n > len(s.buf) {
		for s.w-s.r < len(s.buf) && s.err == nil {
			s.fill()
		}
		bs = s.buf[s.r:s.w]
		err = bufio.ErrBufferFull
		return
	}
	for s.w-s.r < n && s.err == nil {
		s.fill()
	}
	if end := s.r + n; end <= s.w {
		bs = s.buf[s.r:end]
		return
	}
	bs = s.buf[s.r:s.w]
	// Handed over once and cleared, as bufio does: the next call retries the source, so a growing file is followed past its old end.
	err, s.err = s.err, nil
	return
}

func (s *SeekBuffer) Discard(n int) (discarded int, err error) {
	if n < 0 {
		err = bufio.ErrNegativeCount
		return
	}
	for n > 0 {
		if s.r == s.w {
			s.fill()
			if s.r == s.w {
				err, s.err = s.err, nil
				return
			}
		}
		m := min(s.w-s.r, n)
		s.r += m
		discarded += m
		n -= m
	}
	return
}

func (s *SeekBuffer) Seek(offset int64, whence int) (abs int64, err error) {
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = s.bufStart + int64(s.r) + offset
	case io.SeekEnd:
		return s.seekSource(offset, whence)
	default:
		err = errInvalidWhence
		return
	}
	if abs >= s.bufStart && abs <= s.bufStart+int64(s.w) {
		s.r = int(abs - s.bufStart)
		return
	}
	return s.seekSource(abs, io.SeekStart)
}

func (s *SeekBuffer) seekSource(offset int64, whence int) (pos int64, err error) {
	if pos, err = s.rs.Seek(offset, whence); err != nil {
		return
	}
	s.bufStart = pos
	s.r, s.w = 0, 0
	s.err = nil
	return
}

// Slides the unconsumed bytes to the front only once the tail is full: sliding earlier shrinks the in-buffer seek-back window for nothing.
func (s *SeekBuffer) fill() {
	if s.err != nil {
		return
	}
	if s.w >= len(s.buf) {
		if s.r == 0 {
			return
		}
		copy(s.buf, s.buf[s.r:s.w])
		s.bufStart += int64(s.r)
		s.w -= s.r
		s.r = 0
	}
	for range maxEmptyReads {
		var n int
		var err error
		if n, err = s.rs.Read(s.buf[s.w:]); n < 0 {
			panic("tsio.SeekBuffer: reader returned negative count")
		}
		s.w += n
		if err != nil {
			s.err = err
			return
		}
		if n > 0 {
			return
		}
	}
	s.err = io.ErrNoProgress
}
