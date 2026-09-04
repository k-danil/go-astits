package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type Unknown struct {
	Header  Header `json:"_header"`
	Content []byte `json:"_content"`
}

func newDescriptorUnknown(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	d := &Unknown{
		Header: h,
	}
	dd = d

	if h.Length > 0 {
		if d.Content, err = i.NextBytes(int(h.Length)); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *Unknown) CalcLength() int {
	return len(d.Content)
}

func (d *Unknown) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	return append(dst, d.Content...)
}
