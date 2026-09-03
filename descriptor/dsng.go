package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type DSNG struct {
	Header Header `json:"_header"`
	Data   []byte `json:"byte"`
}

func newDescriptorDSNG(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &DSNG{
		Header: h,
	}
	dd = d

	if d.Data, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *DSNG) CalcLength() int {
	return len(d.Data)
}

func (d *DSNG) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, d.Data...)
}
