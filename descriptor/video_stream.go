package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

// VideoStream is the MPEG-2 systems video_stream_descriptor (ISO/IEC 13818-1).
type VideoStream struct {
	Header                    Header
	FrameRateCode             uint8
	ProfileAndLevelIndication uint8
	ChromaFormat              uint8
	MultipleFrameRate         bool
	MPEG1Only                 bool
	ConstrainedParameter      bool
	StillPicture              bool
	FrameRateExtension        bool
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
