package descriptor

import (
	"fmt"

	"github.com/asticode/go-astikit"
)

// DescriptorNetworkName represents a network name descriptor
// Chapter: 6.2.27 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorNetworkName struct {
	Header DescriptorHeader
	Name   []byte
}

func newDescriptorNetworkName(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (dd Descriptor, err error) {
	// Create descriptor
	d := &DescriptorNetworkName{
		Header: h,
	}
	dd = d

	// Name
	if d.Name, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *DescriptorNetworkName) length() uint8 {
	return uint8(len(d.Name))
}

func (d *DescriptorNetworkName) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.Name)

	return written, b.Err()
}
