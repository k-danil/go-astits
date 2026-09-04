package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type ShortSmoothingBuffer struct {
	Reserved   []byte `json:"_reserved"`
	Header     Header `json:"_header"`
	SBSize     uint8  `json:"sb_size"`
	SBLeakRate uint8  `json:"sb_leak_rate"`
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

	if offsetEnd > i.Offset() {
		if d.Reserved, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *ShortSmoothingBuffer) CalcLength() int {
	return 1 + len(d.Reserved)
}

func (d *ShortSmoothingBuffer) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	dst = append(dst, d.SBSize&0x03<<6|d.SBLeakRate&0x3f)
	return append(dst, d.Reserved...)
}
