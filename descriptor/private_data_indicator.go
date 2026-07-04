package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/asticode/go-astikit"
)

// DescriptorPrivateDataIndicator represents a private data Indicator descriptor
type DescriptorPrivateDataIndicator struct {
	Header    DescriptorHeader
	Indicator uint32
}

func newDescriptorPrivateDataIndicator(i *astikit.BytesIterator, h DescriptorHeader, _ int) (dd Descriptor, err error) {
	// Get next bytes
	var bs []byte
	if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	// Create descriptor
	d := &DescriptorPrivateDataIndicator{
		Header:    h,
		Indicator: binary.BigEndian.Uint32(bs),
	}
	dd = d
	return
}

func (*DescriptorPrivateDataIndicator) length() uint8 {
	return 4
}

func (d *DescriptorPrivateDataIndicator) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.Indicator)

	return written, b.Err()
}
