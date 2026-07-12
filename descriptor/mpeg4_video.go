package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// MPEG4Video is the MPEG-2 systems MPEG-4_video_descriptor (ISO/IEC 13818-1).
type MPEG4Video struct {
	Header          Header
	ProfileAndLevel uint8
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
