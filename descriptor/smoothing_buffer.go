package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// SmoothingBuffer is the MPEG-2 systems smoothing_buffer_descriptor (ISO/IEC 13818-1).
type SmoothingBuffer struct {
	SbLeakRate uint32 `json:"sb_leak_rate"` // In units of 400 bits/s
	SbSize     uint32 `json:"sb_size"`      // In bytes
	Header     Header `json:"_header"`
}

func newDescriptorSmoothingBuffer(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(6); err != nil || len(bs) < 6 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &SmoothingBuffer{
		Header:     h,
		SbLeakRate: uint32(bs[0]&0x3f)<<16 | uint32(bs[1])<<8 | uint32(bs[2]),
		SbSize:     uint32(bs[3]&0x3f)<<16 | uint32(bs[4])<<8 | uint32(bs[5]),
	}
	dd = d
	return
}

func (*SmoothingBuffer) CalcLength() int {
	return 6
}

func (d *SmoothingBuffer) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, 0xc0|byte(d.SbLeakRate>>16)&0x3f, byte(d.SbLeakRate>>8), byte(d.SbLeakRate))
	return append(dst, 0xc0|byte(d.SbSize>>16)&0x3f, byte(d.SbSize>>8), byte(d.SbSize))
}
