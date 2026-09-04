package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type MPEG4Text struct {
	TextConfig []byte `json:"textConfig"`
	Header     Header `json:"_header"`
}

func newDescriptorMPEG4Text(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &MPEG4Text{Header: h}
	dd = d

	if i.Offset() < offsetEnd {
		if d.TextConfig, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *MPEG4Text) CalcLength() int {
	return len(d.TextConfig)
}

func (d *MPEG4Text) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	return append(dst, d.TextConfig...)
}
