package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// StereoscopicProgramInfo is the MPEG-2 systems Stereoscopic_program_info_descriptor (ISO/IEC 13818-1).
type StereoscopicProgramInfo struct {
	Header      Header
	ServiceType uint8
}

func newDescriptorStereoscopicProgramInfo(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
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
	return
}

func (*StereoscopicProgramInfo) CalcLength() int {
	return 1
}

func (d *StereoscopicProgramInfo) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, 0xf8|d.ServiceType&0x07)
}
