package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// VideoWindow is the MPEG-2 systems video_window_descriptor (ISO/IEC 13818-1).
type VideoWindow struct {
	Header           Header
	HorizontalOffset uint16
	VerticalOffset   uint16
	WindowPriority   uint8
}

func newDescriptorVideoWindow(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	v := binary.BigEndian.Uint32(bs)
	d := &VideoWindow{
		Header:           h,
		HorizontalOffset: uint16(v >> 18 & 0x3fff),
		VerticalOffset:   uint16(v >> 4 & 0x3fff),
		WindowPriority:   uint8(v & 0xf),
	}
	dd = d
	return
}

func (*VideoWindow) CalcLength() int {
	return 4
}

func (d *VideoWindow) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	v := uint32(d.HorizontalOffset&0x3fff)<<18 | uint32(d.VerticalOffset&0x3fff)<<4 | uint32(d.WindowPriority&0xf)
	return append(dst, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}
