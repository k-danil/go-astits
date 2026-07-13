package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/descriptor/ext"
	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// Extension represents a DVB extension_descriptor (tag 0x7f); the concrete
// sub-descriptor, selected by an extension_descriptor_tag, is held in Body.
// Chapter: 6.2.16 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
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
