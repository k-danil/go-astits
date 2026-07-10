package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// AdaptationFieldData represents an adaptation field data descriptor: a bitmask
// of the data field types carried in the adaptation-field private data.
// Chapter: 6.2.1 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type AdaptationFieldData struct {
	Header     Header
	Identifier uint8
}

func newDescriptorAdaptationFieldData(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	d := &AdaptationFieldData{
		Header: h,
	}
	dd = d

	if d.Identifier, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	return
}

func (d *AdaptationFieldData) CalcLength() int {
	return 1
}

func (d *AdaptationFieldData) Append(dst []byte) []byte {
	return append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()), d.Identifier)
}
