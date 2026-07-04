package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/asticode/go-astikit"
)

// DescriptorSubtitling represents a subtitling descriptor
// Chapter: 6.2.41 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorSubtitling struct {
	Header DescriptorHeader
	Items  []DescriptorSubtitlingItem
}

// DescriptorSubtitlingItem represents subtitling descriptor item
// Chapter: 6.2.41 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorSubtitlingItem struct {
	AncillaryPageID   uint16
	CompositionPageID uint16
	Language          [3]byte
	Type              uint8
}

func newDescriptorSubtitling(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (dd Descriptor, err error) {
	// Create descriptor
	d := &DescriptorSubtitling{
		Header: h,
		Items:  make([]DescriptorSubtitlingItem, (offsetEnd-i.Offset())/8),
	}
	dd = d

	// Loop
	for idx := range d.Items {
		// Language
		var bs []byte
		if bs, err = i.NextBytesNoCopy(3); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		copy(d.Items[idx].Language[:], bs)

		// Get next byte
		var b byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		// Type
		d.Items[idx].Type = b

		// Get next bytes
		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}

		// Composition page ID
		d.Items[idx].CompositionPageID = binary.BigEndian.Uint16(bs)

		// Get next bytes
		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}

		// Ancillary page ID
		d.Items[idx].AncillaryPageID = binary.BigEndian.Uint16(bs)
	}
	return
}

func (d *DescriptorSubtitling) length() uint8 {
	return uint8(8 * len(d.Items))
}

func (d *DescriptorSubtitling) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	for _, item := range d.Items {
		b.Write(item.Language[:])
		b.Write(item.Type)
		b.Write(item.CompositionPageID)
		b.Write(item.AncillaryPageID)
	}

	return written, b.Err()
}
