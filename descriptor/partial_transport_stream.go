package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// PartialTransportStream represents a partial transport stream descriptor: the
// play-out/copy control parameters (rates and buffer) of a partial TS, carried
// in the transmission-info loop of a SIT.
// Chapter: 7.2.1 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type PartialTransportStream struct {
	Header                        Header
	PeakRate                      uint32
	MinimumOverallSmoothingRate   uint32
	MaximumOverallSmoothingBuffer uint16
}

func newDescriptorPartialTransportStream(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	d := &PartialTransportStream{
		Header: h,
	}
	dd = d

	var bs []byte
	if bs, err = i.NextBytesNoCopy(8); err != nil || len(bs) < 8 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d.PeakRate = uint32(bs[0]&0x3f)<<16 | uint32(bs[1])<<8 | uint32(bs[2])
	d.MinimumOverallSmoothingRate = uint32(bs[3]&0x3f)<<16 | uint32(bs[4])<<8 | uint32(bs[5])
	d.MaximumOverallSmoothingBuffer = uint16(bs[6]&0x3f)<<8 | uint16(bs[7])
	return
}

func (d *PartialTransportStream) CalcLength() int {
	return 8
}

func (d *PartialTransportStream) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst,
		0xc0|byte(d.PeakRate>>16)&0x3f, byte(d.PeakRate>>8), byte(d.PeakRate),
		0xc0|byte(d.MinimumOverallSmoothingRate>>16)&0x3f,
		byte(d.MinimumOverallSmoothingRate>>8), byte(d.MinimumOverallSmoothingRate),
		0xc0|byte(d.MaximumOverallSmoothingBuffer>>8)&0x3f, byte(d.MaximumOverallSmoothingBuffer))
}
