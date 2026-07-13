package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// Scrambling represents a scrambling descriptor: the selected mode of the
// scrambling system (see annex E of EN 300 468).
// Chapter: 6.2.32 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
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
