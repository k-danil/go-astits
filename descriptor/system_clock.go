package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
	"github.com/k-danil/go-astits/v3/internal/util"
)

type SystemClock struct {
	Header                          Header `json:"_header"`
	ClockAccuracyInteger            uint8  `json:"clock_accuracy_integer"`
	ClockAccuracyExponent           uint8  `json:"clock_accuracy_exponent"`
	ExternalClockReferenceIndicator bool   `json:"external_clock_reference_indicator"`
}

func newDescriptorSystemClock(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil {
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

	err = rejectTrailingBytes(i, offsetEnd)
	return
}

func (d *SystemClock) CalcLength() int {
	return 2
}

func (d *SystemClock) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	dst = append(dst, util.B2U(d.ExternalClockReferenceIndicator)<<7|1<<6|d.ClockAccuracyInteger&0x3f)
	return append(dst, d.ClockAccuracyExponent&0x7<<5|0x1f)
}
