package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

// J2KVideo is the MPEG-2 systems J2K_video_descriptor (ISO/IEC 13818-1).
type J2KVideo struct {
	PrivateData        []byte `json:"private_data"`
	HorizontalSize     uint32 `json:"horizontal_size"`
	VerticalSize       uint32 `json:"vertical_size"`
	MaxBitRate         uint32 `json:"max_bit_rate"`
	MaxBufferSize      uint32 `json:"max_buffer_size"`
	Header             Header `json:"_header"`
	ProfileAndLevel    uint16 `json:"profile_and_level"`
	DENFrameRate       uint16 `json:"DEN_frame_rate"`
	NUMFrameRate       uint16 `json:"NUM_frame_rate"`
	ColorSpecification uint8  `json:"color_specification"`
	StillMode          bool   `json:"still_mode"`
	InterlacedVideo    bool   `json:"interlaced_video"`
}

func newDescriptorJ2KVideo(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(24); err != nil || len(bs) < 24 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &J2KVideo{
		Header:             h,
		ProfileAndLevel:    binary.BigEndian.Uint16(bs),
		HorizontalSize:     binary.BigEndian.Uint32(bs[2:]),
		VerticalSize:       binary.BigEndian.Uint32(bs[6:]),
		MaxBitRate:         binary.BigEndian.Uint32(bs[10:]),
		MaxBufferSize:      binary.BigEndian.Uint32(bs[14:]),
		DENFrameRate:       binary.BigEndian.Uint16(bs[18:]),
		NUMFrameRate:       binary.BigEndian.Uint16(bs[20:]),
		ColorSpecification: bs[22],
		StillMode:          bs[23]&0x80 > 0,
		InterlacedVideo:    bs[23]&0x40 > 0,
	}
	dd = d

	if i.Offset() < offsetEnd {
		if d.PrivateData, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *J2KVideo) CalcLength() int {
	return 24 + len(d.PrivateData)
}

func (d *J2KVideo) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst,
		byte(d.ProfileAndLevel>>8), byte(d.ProfileAndLevel),
		byte(d.HorizontalSize>>24), byte(d.HorizontalSize>>16), byte(d.HorizontalSize>>8), byte(d.HorizontalSize),
		byte(d.VerticalSize>>24), byte(d.VerticalSize>>16), byte(d.VerticalSize>>8), byte(d.VerticalSize),
		byte(d.MaxBitRate>>24), byte(d.MaxBitRate>>16), byte(d.MaxBitRate>>8), byte(d.MaxBitRate),
		byte(d.MaxBufferSize>>24), byte(d.MaxBufferSize>>16), byte(d.MaxBufferSize>>8), byte(d.MaxBufferSize),
		byte(d.DENFrameRate>>8), byte(d.DENFrameRate),
		byte(d.NUMFrameRate>>8), byte(d.NUMFrameRate),
		d.ColorSpecification,
		util.B2U(d.StillMode)<<7|util.B2U(d.InterlacedVideo)<<6|0x3f,
	)
	return append(dst, d.PrivateData...)
}
