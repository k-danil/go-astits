// Package bitstest: API shape follows asticode/go-astikit (MIT).
package bitstest

import "io"

type Writer struct {
	w     io.Writer
	cache uint8
	bits  int
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

func (w *Writer) WriteBit(set bool) {
	w.cache <<= 1
	if set {
		w.cache |= 1
	}
	w.bits++
	if w.bits == 8 {
		_, _ = w.w.Write([]byte{w.cache})
		w.cache = 0
		w.bits = 0
	}
}

func (w *Writer) WriteBits(v uint64, n int) {
	for i := n - 1; i >= 0; i-- {
		w.WriteBit(v>>i&1 == 1)
	}
}

func (w *Writer) Write(v any) error {
	switch t := v.(type) {
	case string:
		for _, r := range t {
			w.WriteBit(r == '1')
		}
	case bool:
		w.WriteBit(t)
	case uint8:
		w.WriteBits(uint64(t), 8)
	case uint16:
		w.WriteBits(uint64(t), 16)
	case uint32:
		w.WriteBits(uint64(t), 32)
	case uint64:
		w.WriteBits(t, 64)
	case []byte:
		for _, b := range t {
			w.WriteBits(uint64(b), 8)
		}
	default:
		panic("bitstest: unsupported type")
	}
	return nil
}

func (w *Writer) WriteN(v any, n int) error {
	switch t := v.(type) {
	case uint8:
		w.WriteBits(uint64(t), n)
	case uint16:
		w.WriteBits(uint64(t), n)
	case uint32:
		w.WriteBits(uint64(t), n)
	case uint64:
		w.WriteBits(t, n)
	default:
		panic("bitstest: unsupported type")
	}
	return nil
}
