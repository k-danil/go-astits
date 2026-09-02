// Package bytesiter is a minimal byte iterator for the cold parse paths
// (PSI tables, descriptors, DVB time): hot paths parse slices directly.
// The API shape follows asticode/go-astikit (MIT, same author as the upstream fork).
package bytesiter

import (
	"github.com/k-danil/go-astits/v2/internal/errclass"
	"github.com/k-danil/go-astits/v2/ts"
)

var ErrNoBytesLeft = errclass.New("astits: not enough bytes", ts.ErrInvalidData)

type Iterator struct {
	bs     []byte
	offset int
	limit  int
}

func New(bs []byte) *Iterator {
	return &Iterator{bs: bs, limit: len(bs)}
}

func (i *Iterator) NextByte() (b byte, err error) {
	if i.offset < 0 || i.offset >= i.limit {
		return 0, ErrNoBytesLeft
	}
	b = i.bs[i.offset]
	i.offset++
	return
}

// NextBytesNoCopy returns the next n bytes as a view into the underlying slice.
func (i *Iterator) NextBytesNoCopy(n int) (bs []byte, err error) {
	if n < 0 || i.offset < 0 || i.offset+n > i.limit {
		return nil, ErrNoBytesLeft
	}
	bs = i.bs[i.offset : i.offset+n]
	i.offset += n
	return
}

func (i *Iterator) NextBytes(n int) (bs []byte, err error) {
	var v []byte
	if v, err = i.NextBytesNoCopy(n); err != nil {
		return
	}
	bs = make([]byte, n)
	copy(bs, v)
	return
}

func (i *Iterator) Skip(n int) {
	i.offset += n
}

func (i *Iterator) Seek(offset int) {
	i.offset = offset
}

func (i *Iterator) Offset() int {
	return i.offset
}

// Len is the readable end: the limit, not the backing slice.
func (i *Iterator) Len() int {
	return i.limit
}

// Limit caps reads at end (clamped to the backing slice) and returns the
// previous cap, so a nested scope restores it. A section body parsed under its
// own end cannot read into the next section, whatever its inner lengths claim.
func (i *Iterator) Limit(end int) (prev int) {
	prev = i.limit
	i.limit = min(end, len(i.bs))
	return
}

func (i *Iterator) HasBytesLeft() bool {
	return i.offset < i.limit
}

// Bytes returns the unread remainder without advancing.
func (i *Iterator) Bytes() []byte {
	if i.offset < 0 || i.offset >= i.limit {
		return nil
	}
	return i.bs[i.offset:i.limit]
}
