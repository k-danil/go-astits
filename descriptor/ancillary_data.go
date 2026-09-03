package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type AncillaryData struct {
	Header     Header `json:"_header"`
	Identifier uint8  `json:"ancillary_data_identifier"`
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
