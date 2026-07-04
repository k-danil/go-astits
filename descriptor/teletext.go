package descriptor

import (
	"fmt"

	"github.com/asticode/go-astikit"
)

// Teletext types
// Chapter: 6.2.43 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
const (
	TeletextTypeAdditionalInformationPage                    = 0x3
	TeletextTypeInitialTeletextPage                          = 0x1
	TeletextTypeProgramSchedulePage                          = 0x4
	TeletextTypeTeletextSubtitlePage                         = 0x2
	TeletextTypeTeletextSubtitlePageForHearingImpairedPeople = 0x5
)

// DescriptorTeletext represents a teletext descriptor
// Chapter: 6.2.43 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorTeletext struct {
	Header DescriptorHeader
	Items  []DescriptorTeletextItem
}

// DescriptorTeletextItem represents a teletext descriptor item
// Chapter: 6.2.43 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorTeletextItem struct {
	Language [3]byte
	Magazine uint8
	Page     uint8
	Type     uint8
}

func newDescriptorTeletext(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (dd Descriptor, err error) {
	// Create descriptor
	d := &DescriptorTeletext{
		Header: h,
		Items:  make([]DescriptorTeletextItem, (offsetEnd-i.Offset())/5),
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
		d.Items[idx].Type = b >> 3

		// Magazine
		d.Items[idx].Magazine = b & 0x7

		// Get next byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		// Page
		d.Items[idx].Page = b>>4*10 + b&0xf
	}
	return
}

func (d *DescriptorTeletext) length() uint8 {
	return uint8(5 * len(d.Items))
}

func (d *DescriptorTeletext) write(w *astikit.BitsWriter) (int, error) {
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
		b.WriteN(item.Type, 5)
		b.WriteN(item.Magazine, 3)
		b.WriteN(item.Page/10, 4)
		b.WriteN(item.Page%10, 4)
	}

	return written, b.Err()
}
