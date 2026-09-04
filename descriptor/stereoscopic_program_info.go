package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type StereoscopicProgramInfo struct {
	Header      Header `json:"_header"`
	ServiceType uint8  `json:"service_type"`
}

func newDescriptorStereoscopicProgramInfo(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d := &StereoscopicProgramInfo{
		Header:      h,
		ServiceType: b & 0x07,
	}
	dd = d

	err = rejectTrailingBytes(i, offsetEnd)
	return
}

func (d *StereoscopicProgramInfo) CalcLength() int {
	return 1
}

func (d *StereoscopicProgramInfo) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	return append(dst, 0xf8|d.ServiceType&0x07)
}
