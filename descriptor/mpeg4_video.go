package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type MPEG4Video struct {
	Header          Header `json:"_header"`
	ProfileAndLevel uint8  `json:"profile_and_level"`
}

func newDescriptorMPEG4Video(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d := &MPEG4Video{
		Header:          h,
		ProfileAndLevel: b,
	}
	dd = d
	return
}

func (*MPEG4Video) CalcLength() int {
	return 1
}

func (d *MPEG4Video) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, d.ProfileAndLevel)
}
