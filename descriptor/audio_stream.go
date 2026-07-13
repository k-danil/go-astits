package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

// AudioStream is the MPEG-2 systems audio_stream_descriptor (ISO/IEC 13818-1).
type AudioStream struct {
	Header                     Header `json:"_header"`
	Layer                      uint8  `json:"layer"`
	FreeFormatFlag             bool   `json:"free_format_flag"`
	ID                         bool   `json:"ID"`
	VariableRateAudioIndicator bool   `json:"variable_rate_audio_indicator"`
}

func newDescriptorAudioStream(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	d := &AudioStream{
		Header:                     h,
		Layer:                      b >> 4 & 0x3,
		FreeFormatFlag:             b&0x80 > 0,
		ID:                         b&0x40 > 0,
		VariableRateAudioIndicator: b&0x08 > 0,
	}
	dd = d
	return
}

func (*AudioStream) CalcLength() int {
	return 1
}

func (d *AudioStream) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, util.B2U(d.FreeFormatFlag)<<7|util.B2U(d.ID)<<6|(d.Layer&0x3)<<4|util.B2U(d.VariableRateAudioIndicator)<<3|0x7)
}
