package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/internal/bytesiter"
)

type Unknown struct {
	Header  Header
	Content []byte
}

func newDescriptorUnknown(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	// Create descriptor
	d := &Unknown{
		Header: h,
	}
	dd = d

	// Get next bytes
	if d.Content, err = i.NextBytes(int(h.Length)); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *Unknown) CalcLength() int {
	return len(d.Content)
}

func (d *Unknown) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, d.Content...)
}
