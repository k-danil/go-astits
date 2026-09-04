package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

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
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	dst = append(dst, d.AuxVideoCodedStreamType)
	return append(dst, d.SIRBSP...)
}
