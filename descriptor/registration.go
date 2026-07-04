package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/asticode/go-astikit"
)

// DescriptorRegistration represents a registration descriptor
// Page: 84 | http://ecee.colorado.edu/~ecen5653/ecen5653/papers/iso13818-1.pdf
type DescriptorRegistration struct {
	AdditionalIdentificationInfo []byte
	FormatIdentifier             uint32
	Header                       DescriptorHeader
}

func newDescriptorRegistration(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (dd Descriptor, err error) {
	// Get next bytes
	var bs []byte
	if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	// Create descriptor
	d := &DescriptorRegistration{
		Header:           h,
		FormatIdentifier: binary.BigEndian.Uint32(bs),
	}
	dd = d

	// Additional identification info
	if i.Offset() < offsetEnd {
		if d.AdditionalIdentificationInfo, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *DescriptorRegistration) length() uint8 {
	return uint8(4 + len(d.AdditionalIdentificationInfo))
}

func (d *DescriptorRegistration) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.FormatIdentifier)
	b.Write(d.AdditionalIdentificationInfo)

	return written, b.Err()
}
