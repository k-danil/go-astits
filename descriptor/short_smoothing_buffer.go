package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// ShortSmoothingBuffer represents a short smoothing buffer descriptor: the
// smoothing buffer size and leak rate signalling a service's bit-rate.
// Chapter: 6.2.38 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type ShortSmoothingBuffer struct {
	Reserved   []byte
	Header     Header
	SBSize     uint8
	SBLeakRate uint8
}

func newDescriptorShortSmoothingBuffer(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &ShortSmoothingBuffer{
		Header: h,
	}
	dd = d

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.SBSize = b >> 6 & 0x03
	d.SBLeakRate = b & 0x3f

	if d.Reserved, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *ShortSmoothingBuffer) CalcLength() int {
	return 1 + len(d.Reserved)
}

func (d *ShortSmoothingBuffer) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, d.SBSize&0x03<<6|d.SBLeakRate&0x3f)
	return append(dst, d.Reserved...)
}
