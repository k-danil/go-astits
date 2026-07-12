package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

// SystemClock is the MPEG-2 systems system_clock_descriptor (ISO/IEC 13818-1).
type SystemClock struct {
	Header                          Header
	ClockAccuracyInteger            uint8
	ClockAccuracyExponent           uint8
	ExternalClockReferenceIndicator bool
}

func newDescriptorSystemClock(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &SystemClock{
		Header:                          h,
		ClockAccuracyInteger:            bs[0] & 0x3f,
		ClockAccuracyExponent:           bs[1] >> 5 & 0x7,
		ExternalClockReferenceIndicator: bs[0]&0x80 > 0,
	}
	dd = d
	return
}

func (*SystemClock) CalcLength() int {
	return 2
}

func (d *SystemClock) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, util.B2U(d.ExternalClockReferenceIndicator)<<7|1<<6|d.ClockAccuracyInteger&0x3f)
	return append(dst, d.ClockAccuracyExponent&0x7<<5|0x1f)
}
