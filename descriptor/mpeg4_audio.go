package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// MPEG4Audio is the MPEG-2 systems MPEG-4_audio_descriptor (ISO/IEC 13818-1).
type MPEG4Audio struct {
	Header          Header `json:"_header"`
	ProfileAndLevel uint8  `json:"profile_and_level"`
}

func newDescriptorMPEG4Audio(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d := &MPEG4Audio{
		Header:          h,
		ProfileAndLevel: b,
	}
	dd = d
	return
}

func (*MPEG4Audio) CalcLength() int {
	return 1
}

func (d *MPEG4Audio) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, d.ProfileAndLevel)
}
