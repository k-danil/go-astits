package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// MultiplexBuffer is the MPEG-2 systems MultiplexBuffer descriptor (ISO/IEC 13818-1).
type MultiplexBuffer struct {
	MBBufferSize uint32 // bytes
	TBLeakRate   uint32 // units of 400 bit/s
	Header       Header
}

func newDescriptorMultiplexBuffer(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(6); err != nil || len(bs) < 6 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &MultiplexBuffer{
		Header:       h,
		MBBufferSize: uint32(bs[0])<<16 | uint32(bs[1])<<8 | uint32(bs[2]),
		TBLeakRate:   uint32(bs[3])<<16 | uint32(bs[4])<<8 | uint32(bs[5]),
	}
	dd = d
	return
}

func (*MultiplexBuffer) CalcLength() int {
	return 6
}

func (d *MultiplexBuffer) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, byte(d.MBBufferSize>>16), byte(d.MBBufferSize>>8), byte(d.MBBufferSize))
	return append(dst, byte(d.TBLeakRate>>16), byte(d.TBLeakRate>>8), byte(d.TBLeakRate))
}
