package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// AncillaryData represents an ancillary data descriptor: a bitmask of the
// ancillary data types present in an audio elementary stream.
// Chapter: 6.2.2 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type AncillaryData struct {
	Header     Header
	Identifier uint8
}

func newDescriptorAncillaryData(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	d := &AncillaryData{
		Header: h,
	}
	dd = d

	if d.Identifier, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	return
}

func (d *AncillaryData) CalcLength() int {
	return 1
}

func (d *AncillaryData) Append(dst []byte) []byte {
	return append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()), d.Identifier)
}
