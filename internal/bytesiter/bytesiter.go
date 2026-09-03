// Package bytesiter: API shape follows asticode/go-astikit (MIT).
package bytesiter

import (
	"github.com/k-danil/go-astits/v3/internal/errclass"
	"github.com/k-danil/go-astits/v3/ts"
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

// Limit caps reads at end and returns the previous cap for the caller to restore, so a section body cannot read past its own end whatever its inner lengths claim.
func (i *Iterator) Limit(end int) (prev int) {
	prev = i.limit
	i.limit = min(end, len(i.bs))
	return
}

func (i *Iterator) HasBytesLeft() bool {
	return i.offset < i.limit
}

func (i *Iterator) Bytes() []byte {
	if i.offset < 0 || i.offset >= i.limit {
		return nil
	}
	return i.bs[i.offset:i.limit]
}
