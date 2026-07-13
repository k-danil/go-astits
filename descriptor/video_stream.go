package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

// VideoStream is the MPEG-2 systems video_stream_descriptor (ISO/IEC 13818-1).
type VideoStream struct {
	Header                    Header `json:"_header"`
	FrameRateCode             uint8  `json:"frame_rate_code"`
	ProfileAndLevelIndication uint8  `json:"profile_and_level_indication"`
	ChromaFormat              uint8  `json:"chroma_format"`
	MultipleFrameRate         bool   `json:"multiple_frame_rate_flag"`
	MPEG1Only                 bool   `json:"MPEG_1_only_flag"`
	ConstrainedParameter      bool   `json:"constrained_parameter_flag"`
	StillPicture              bool   `json:"still_picture_flag"`
	FrameRateExtension        bool   `json:"frame_rate_extension_flag"`
}

func newDescriptorVideoStream(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	d := &VideoStream{
		Header:               h,
		MultipleFrameRate:    b&0x80 > 0,
		FrameRateCode:        b >> 3 & 0x0f,
		MPEG1Only:            b&0x04 > 0,
		ConstrainedParameter: b&0x02 > 0,
		StillPicture:         b&0x01 > 0,
	}
	dd = d

	if !d.MPEG1Only {
		var bs []byte
		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		d.ProfileAndLevelIndication = bs[0]
		d.ChromaFormat = bs[1] >> 6 & 0x03
		d.FrameRateExtension = bs[1]&0x20 > 0
	}
	return
}

func (d *VideoStream) CalcLength() int {
	if d.MPEG1Only {
		return 1
	}
	return 3
}

func (d *VideoStream) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, util.B2U(d.MultipleFrameRate)<<7|d.FrameRateCode&0x0f<<3|util.B2U(d.MPEG1Only)<<2|util.B2U(d.ConstrainedParameter)<<1|util.B2U(d.StillPicture))

	if !d.MPEG1Only {
		dst = append(dst, d.ProfileAndLevelIndication)
		dst = append(dst, d.ChromaFormat&0x03<<6|util.B2U(d.FrameRateExtension)<<5|0x1f)
	}
	return dst
}
