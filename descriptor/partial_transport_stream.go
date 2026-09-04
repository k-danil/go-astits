package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type PartialTransportStream struct {
	Header                        Header `json:"_header"`
	PeakRate                      uint32 `json:"peak_rate"`
	MinimumOverallSmoothingRate   uint32 `json:"minimum_overall_smoothing_rate"`
	MaximumOverallSmoothingBuffer uint16 `json:"maximum_overall_smoothing_buffer"`
}

func newDescriptorPartialTransportStream(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &PartialTransportStream{
		Header: h,
	}
	dd = d

	var bs []byte
	if bs, err = i.NextBytesNoCopy(8); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d.PeakRate = uint32(bs[0]&0x3f)<<16 | uint32(bs[1])<<8 | uint32(bs[2])
	d.MinimumOverallSmoothingRate = uint32(bs[3]&0x3f)<<16 | uint32(bs[4])<<8 | uint32(bs[5])
	d.MaximumOverallSmoothingBuffer = uint16(bs[6]&0x3f)<<8 | uint16(bs[7])

	err = rejectTrailingBytes(i, offsetEnd)
	return
}

func (d *PartialTransportStream) CalcLength() int {
	return 8
}

func (d *PartialTransportStream) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	return append(dst,
		0xc0|byte(d.PeakRate>>16)&0x3f, byte(d.PeakRate>>8), byte(d.PeakRate),
		0xc0|byte(d.MinimumOverallSmoothingRate>>16)&0x3f,
		byte(d.MinimumOverallSmoothingRate>>8), byte(d.MinimumOverallSmoothingRate),
		0xc0|byte(d.MaximumOverallSmoothingBuffer>>8)&0x3f, byte(d.MaximumOverallSmoothingBuffer))
}
