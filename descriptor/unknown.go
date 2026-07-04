package descriptor

import (
	"fmt"

	"github.com/asticode/go-astikit"
)

type DescriptorUnknown struct {
	Header  DescriptorHeader
	Content []byte
}

func newDescriptorUnknown(i *astikit.BytesIterator, h DescriptorHeader, _ int) (dd Descriptor, err error) {
	// Create descriptor
	d := &DescriptorUnknown{
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

func (d *DescriptorUnknown) length() uint8 {
	return uint8(len(d.Content))
}

func (d *DescriptorUnknown) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.Content)

	return written, b.Err()
}
