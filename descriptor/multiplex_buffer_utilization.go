package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// MultiplexBufferUtilization is the MPEG-2 systems multiplex_buffer_utilization_descriptor (ISO/IEC 13818-1).
type MultiplexBufferUtilization struct {
	Header     Header
	LowerBound uint16
	UpperBound uint16
	BoundValid bool
}

func newDescriptorMultiplexBufferUtilization(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &MultiplexBufferUtilization{
		Header:     h,
		BoundValid: bs[0]&0x80 > 0,
		LowerBound: uint16(bs[0]&0x7f)<<8 | uint16(bs[1]),
		UpperBound: uint16(bs[2]&0x7f)<<8 | uint16(bs[3]),
	}
	dd = d
	return
}

func (*MultiplexBufferUtilization) CalcLength() int {
	return 4
}

func (d *MultiplexBufferUtilization) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	lowerHi := byte(d.LowerBound>>8) & 0x7f
	if d.BoundValid {
		lowerHi |= 0x80
	}
	return append(dst,
		lowerHi, byte(d.LowerBound),
		0x80|byte(d.UpperBound>>8)&0x7f, byte(d.UpperBound),
	)
}
