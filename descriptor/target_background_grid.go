package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// TargetBackgroundGrid is the MPEG-2 systems target_background_grid_descriptor (ISO/IEC 13818-1).
type TargetBackgroundGrid struct {
	Header                 Header
	HorizontalSize         uint16
	VerticalSize           uint16
	AspectRatioInformation uint8
}

func newDescriptorTargetBackgroundGrid(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &TargetBackgroundGrid{
		Header:                 h,
		HorizontalSize:         uint16(bs[0])<<6 | uint16(bs[1])>>2,
		VerticalSize:           uint16(bs[1]&0x03)<<12 | uint16(bs[2])<<4 | uint16(bs[3])>>4,
		AspectRatioInformation: bs[3] & 0x0f,
	}
	dd = d
	return
}

func (*TargetBackgroundGrid) CalcLength() int {
	return 4
}

func (d *TargetBackgroundGrid) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	h := d.HorizontalSize & 0x3fff
	v := d.VerticalSize & 0x3fff
	return append(dst,
		byte(h>>6),
		byte(h<<2)|byte(v>>12),
		byte(v>>4),
		byte(v<<4)|d.AspectRatioInformation&0x0f,
	)
}
