package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// MPEG2AACAudio is the MPEG-2 systems MPEG-2_AAC_audio_descriptor (ISO/IEC 13818-1).
type MPEG2AACAudio struct {
	Header                Header
	Profile               uint8
	ChannelConfiguration  uint8
	AdditionalInformation uint8
}

func newDescriptorMPEG2AACAudio(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &MPEG2AACAudio{
		Header:                h,
		Profile:               bs[0],
		ChannelConfiguration:  bs[1],
		AdditionalInformation: bs[2],
	}
	dd = d
	return
}

func (*MPEG2AACAudio) CalcLength() int {
	return 3
}

func (d *MPEG2AACAudio) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, d.Profile, d.ChannelConfiguration, d.AdditionalInformation)
}
