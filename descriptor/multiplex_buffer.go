package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type MultiplexBuffer struct {
	MBBufferSize uint32 `json:"MB_buffer_size"` // bytes
	TBLeakRate   uint32 `json:"TB_leak_rate"`   // units of 400 bit/s
	Header       Header `json:"_header"`
}

func newDescriptorMultiplexBuffer(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(6); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &MultiplexBuffer{
		Header:       h,
		MBBufferSize: uint32(bs[0])<<16 | uint32(bs[1])<<8 | uint32(bs[2]),
		TBLeakRate:   uint32(bs[3])<<16 | uint32(bs[4])<<8 | uint32(bs[5]),
	}
	dd = d

	err = rejectTrailingBytes(i, offsetEnd)
	return
}

func (d *MultiplexBuffer) CalcLength() int {
	return 6
}

func (d *MultiplexBuffer) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	dst = append(dst, byte(d.MBBufferSize>>16), byte(d.MBBufferSize>>8), byte(d.MBBufferSize))
	return append(dst, byte(d.TBLeakRate>>16), byte(d.TBLeakRate>>8), byte(d.TBLeakRate))
}
