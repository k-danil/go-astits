package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

// StereoscopicVideoInfo is the MPEG-2 systems stereoscopic_video_info_descriptor (ISO/IEC 13818-1).
type StereoscopicVideoInfo struct {
	Header                     Header `json:"_header"`
	HorizontalUpsamplingFactor uint8  `json:"horizontal_upsampling_factor"`
	VerticalUpsamplingFactor   uint8  `json:"vertical_upsampling_factor"`
	BaseVideoFlag              bool   `json:"base_video_flag"`
	LeftviewFlag               bool   `json:"leftview_flag"`
	UsableAs2D                 bool   `json:"usable_as_2D"`
}

func newDescriptorStereoscopicVideoInfo(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	d := &StereoscopicVideoInfo{
		Header:        h,
		BaseVideoFlag: b&0x1 > 0,
	}
	dd = d

	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	if d.BaseVideoFlag {
		d.LeftviewFlag = b&0x1 > 0
		return
	}

	d.UsableAs2D = b&0x1 > 0

	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.HorizontalUpsamplingFactor = b >> 4 & 0xf
	d.VerticalUpsamplingFactor = b & 0xf
	return
}

func (d *StereoscopicVideoInfo) CalcLength() int {
	if d.BaseVideoFlag {
		return 2
	}
	return 3
}

func (d *StereoscopicVideoInfo) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, 0xfe|util.B2U(d.BaseVideoFlag))

	if d.BaseVideoFlag {
		return append(dst, 0xfe|util.B2U(d.LeftviewFlag))
	}

	dst = append(dst, 0xfe|util.B2U(d.UsableAs2D))
	return append(dst, d.HorizontalUpsamplingFactor&0xf<<4|d.VerticalUpsamplingFactor&0xf)
}
