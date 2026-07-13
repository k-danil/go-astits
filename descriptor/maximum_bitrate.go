package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// MaximumBitrate represents a maximum bitrate descriptor
type MaximumBitrate struct {
	Bitrate uint32 `json:"bit_rate"` // In bytes/second
	Header  Header `json:"_header"`
}

func newDescriptorMaximumBitrate(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &MaximumBitrate{
		Header:  h,
		Bitrate: (uint32(bs[0]&0x3f)<<16 | uint32(bs[1])<<8 | uint32(bs[2])) * 50,
	}
	dd = d
	return
}

func (*MaximumBitrate) CalcLength() int {
	return 3
}

func (d *MaximumBitrate) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	v := d.Bitrate / 50
	return append(dst, 0xc0|byte(v>>16)&0x3f, byte(v>>8), byte(v))
}
