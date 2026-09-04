package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type AdaptationFieldData struct {
	Header     Header `json:"_header"`
	Identifier uint8  `json:"adaptation_field_data_identifier"`
}

func newDescriptorAdaptationFieldData(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &AdaptationFieldData{
		Header: h,
	}
	dd = d

	if d.Identifier, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	err = rejectTrailingBytes(i, offsetEnd)
	return
}

func (d *AdaptationFieldData) CalcLength() int {
	return 1
}

func (d *AdaptationFieldData) Append(dst []byte) []byte {
	return append(dst, uint8(d.Tag()), uint8(d.CalcLength()), d.Identifier)
}
