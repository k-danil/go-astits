package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type MaximumBitrate struct {
	// In bytes/second: writing rounds down to a multiple of 50 and saturates at 209715150.
	Bitrate uint32 `json:"bit_rate"`
	Header  Header `json:"_header"`
}

const (
	maximumBitrateUnit = 50
	maximumBitrateMax  = 0x3fffff
)

func newDescriptorMaximumBitrate(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &MaximumBitrate{
		Header:  h,
		Bitrate: (uint32(bs[0]&0x3f)<<16 | uint32(bs[1])<<8 | uint32(bs[2])) * maximumBitrateUnit,
	}
	dd = d

	err = rejectTrailingBytes(i, offsetEnd)
	return
}

func (d *MaximumBitrate) CalcLength() int {
	return 3
}

func (d *MaximumBitrate) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	v := min(d.Bitrate/maximumBitrateUnit, maximumBitrateMax)
	return append(dst, 0xc0|byte(v>>16)&0x3f, byte(v>>8), byte(v))
}
