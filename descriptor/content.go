package descriptor

import (
	"fmt"

	"github.com/asticode/go-astikit"
)

// DescriptorContent represents a content descriptor
// Chapter: 6.2.9 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorContent struct {
	Header DescriptorHeader
	Items  []DescriptorContentItem
}

// DescriptorContentItem represents a content item descriptor
// Chapter: 6.2.9 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorContentItem struct {
	ContentNibbleLevel1 uint8
	ContentNibbleLevel2 uint8
	UserByte            uint8
}

func newDescriptorContent(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (dd Descriptor, err error) {
	// Init
	d := &DescriptorContent{
		Header: h,
	}
	dd = d

	// Add items
	for i.Offset() < offsetEnd {
		// Get next bytes
		var bs []byte
		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}

		// Append item
		d.Items = append(d.Items, DescriptorContentItem{
			ContentNibbleLevel1: bs[0] >> 4,
			ContentNibbleLevel2: bs[0] & 0xf,
			UserByte:            bs[1],
		})
	}
	return
}

func (d *DescriptorContent) length() uint8 {
	return uint8(2 * len(d.Items))
}

func (d *DescriptorContent) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	for _, item := range d.Items {
		b.WriteN(item.ContentNibbleLevel1, 4)
		b.WriteN(item.ContentNibbleLevel2, 4)
		b.Write(item.UserByte)
	}

	return written, b.Err()
}
