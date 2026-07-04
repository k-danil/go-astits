// Package bytesiter is a minimal byte iterator for the cold parse paths
// (PSI tables, descriptors, DVB time): hot paths parse slices directly.
// The API shape follows asticode/go-astikit (MIT, same author as the upstream fork).
package bytesiter

import "errors"

var ErrNoBytesLeft = errors.New("astits: not enough bytes")

type Iterator struct {
	bs     []byte
	offset int
}

func New(bs []byte) *Iterator {
	return &Iterator{bs: bs}
}

func (i *Iterator) NextByte() (b byte, err error) {
	if i.offset < 0 || i.offset >= len(i.bs) {
		return 0, ErrNoBytesLeft
	}
	b = i.bs[i.offset]
	i.offset++
	return
}

// NextBytesNoCopy returns the next n bytes as a view into the underlying slice.
func (i *Iterator) NextBytesNoCopy(n int) (bs []byte, err error) {
	if n < 0 || i.offset < 0 || i.offset+n > len(i.bs) {
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

func (i *Iterator) Len() int {
	return len(i.bs)
}

func (i *Iterator) HasBytesLeft() bool {
	return i.offset < len(i.bs)
}

// Bytes returns the unread remainder without advancing.
func (i *Iterator) Bytes() []byte {
	if i.offset < 0 || i.offset >= len(i.bs) {
		return nil
	}
	return i.bs[i.offset:]
}
