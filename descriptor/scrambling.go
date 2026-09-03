package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type Scrambling struct {
	Header Header `json:"_header"`
	Mode   uint8  `json:"scrambling_mode"`
}

func newDescriptorScrambling(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	d := &Scrambling{
		Header: h,
	}
	dd = d

	if d.Mode, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	return
}

func (d *Scrambling) CalcLength() int {
	return 1
}

func (d *Scrambling) Append(dst []byte) []byte {
	return append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()), d.Mode)
}
