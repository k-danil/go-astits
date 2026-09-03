package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type Stuffing struct {
	Header Header `json:"_header"`
	Data   []byte `json:"stuffing_byte"`
}

func newDescriptorStuffing(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &Stuffing{
		Header: h,
	}
	dd = d

	if d.Data, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *Stuffing) CalcLength() int {
	return len(d.Data)
}

func (d *Stuffing) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, d.Data...)
}
