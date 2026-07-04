package descriptor

import (
	"fmt"

	"github.com/asticode/go-astikit"
)

// DescriptorStreamIdentifier represents a stream identifier descriptor
// Chapter: 6.2.39 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorStreamIdentifier struct {
	Header       DescriptorHeader
	ComponentTag uint8
}

func newDescriptorStreamIdentifier(i *astikit.BytesIterator, h DescriptorHeader, _ int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d := &DescriptorStreamIdentifier{
		Header:       h,
		ComponentTag: b,
	}
	dd = d
	return
}

func (*DescriptorStreamIdentifier) length() uint8 {
	return 1
}

func (d *DescriptorStreamIdentifier) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.ComponentTag)

	return written, b.Err()
}
