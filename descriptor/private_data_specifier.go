package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/asticode/go-astikit"
)

// DescriptorPrivateDataSpecifier represents a private data specifier descriptor
type DescriptorPrivateDataSpecifier struct {
	Header    DescriptorHeader
	Specifier uint32
}

func newDescriptorPrivateDataSpecifier(i *astikit.BytesIterator, h DescriptorHeader, _ int) (dd Descriptor, err error) {
	// Get next bytes
	var bs []byte
	if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	// Create descriptor
	d := &DescriptorPrivateDataSpecifier{
		Header:    h,
		Specifier: binary.BigEndian.Uint32(bs),
	}
	dd = d
	return
}

func (*DescriptorPrivateDataSpecifier) length() uint8 {
	return 4
}

func (d *DescriptorPrivateDataSpecifier) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.Specifier)

	return written, b.Err()
}
