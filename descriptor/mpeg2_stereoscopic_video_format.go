package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type MPEG2StereoscopicVideoFormat struct {
	Header             Header `json:"_header"`
	ArrangementType    uint8  `json:"arrangement_type"`
	HasArrangementType bool   `json:"stereo_video_arrangement_type_present"`
}

func newDescriptorMPEG2StereoscopicVideoFormat(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	d := &MPEG2StereoscopicVideoFormat{
		Header:             h,
		HasArrangementType: b&0x80 > 0,
	}
	if d.HasArrangementType {
		d.ArrangementType = b & 0x7f
	}
	dd = d
	return
}

func (*MPEG2StereoscopicVideoFormat) CalcLength() int {
	return 1
}

func (d *MPEG2StereoscopicVideoFormat) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	b := byte(0x7f)
	if d.HasArrangementType {
		b = 0x80 | d.ArrangementType&0x7f
	}
	return append(dst, b)
}
