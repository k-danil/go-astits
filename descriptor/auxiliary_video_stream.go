package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// AuxiliaryVideoStream is the MPEG-2 systems Auxiliary_video_stream_descriptor (ISO/IEC 13818-1).
type AuxiliaryVideoStream struct {
	SIRBSP                  []byte `json:"si_rbsp"`
	Header                  Header `json:"_header"`
	AuxVideoCodedStreamType uint8  `json:"aux_video_codedstreamtype"`
}

func newDescriptorAuxiliaryVideoStream(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	d := &AuxiliaryVideoStream{
		Header:                  h,
		AuxVideoCodedStreamType: b,
	}
	dd = d

	if i.Offset() < offsetEnd {
		if d.SIRBSP, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *AuxiliaryVideoStream) CalcLength() int {
	return 1 + len(d.SIRBSP)
}

func (d *AuxiliaryVideoStream) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, d.AuxVideoCodedStreamType)
	return append(dst, d.SIRBSP...)
}
