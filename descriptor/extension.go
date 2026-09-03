package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/descriptor/ext"
	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type Extension struct {
	Body   ext.Body `json:"_body"`
	Header Header   `json:"_header"`
}

func newDescriptorExtension(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	d := &Extension{Header: h}
	if d.Body, err = ext.Parse(i, ext.Tag(b), offsetEnd); err != nil {
		return
	}
	dd = d
	return
}

func (d *Extension) CalcLength() int {
	return 1 + d.Body.CalcLength()
}

func (d *Extension) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, uint8(d.Body.Tag()))
	return d.Body.Append(dst)
}
