package descriptor

import (
	"fmt"

	"github.com/asticode/go-astikit"
)

// DescriptorExtendedEvent represents an extended event descriptor
// Chapter: 6.2.15 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorExtendedEvent struct {
	Text                 []byte
	Items                []DescriptorExtendedEventItem
	Header               DescriptorHeader
	ISO639LanguageCode   [3]byte
	LastDescriptorNumber uint8
	Number               uint8
}

// DescriptorExtendedEventItem represents an extended event item descriptor
// Chapter: 6.2.15 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorExtendedEventItem struct {
	Content     []byte
	Description []byte
}

func newDescriptorExtendedEvent(i *astikit.BytesIterator, h DescriptorHeader, _ int) (dd Descriptor, err error) {
	// Init
	d := &DescriptorExtendedEvent{
		Header: h,
	}
	dd = d

	// Get next byte
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Number
	d.Number = b >> 4

	// Last descriptor number
	d.LastDescriptorNumber = b & 0xf

	// ISO639 language code
	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	copy(d.ISO639LanguageCode[:], bs)

	// Get next byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Items length
	itemsLength := int(b)

	// Items
	offsetEnd := i.Offset() + itemsLength
	for i.Offset() < offsetEnd {
		// Create item
		var item DescriptorExtendedEventItem
		if err = item.newDescriptorExtendedEventItem(i); err != nil {
			err = fmt.Errorf("astits: creating extended event item failed: %w", err)
			return
		}

		// Append item
		d.Items = append(d.Items, item)
	}

	// Get next byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Text length
	textLength := int(b)

	// Text
	if d.Text, err = i.NextBytes(textLength); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *DescriptorExtendedEventItem) newDescriptorExtendedEventItem(i *astikit.BytesIterator) (err error) {
	// Get next byte
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Description length
	descriptionLength := int(b)

	// Description
	if d.Description, err = i.NextBytes(descriptionLength); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	// Get next byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Content length
	contentLength := int(b)

	// Content
	if d.Content, err = i.NextBytes(contentLength); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *DescriptorExtendedEvent) length() uint8 {
	ret := 1 + 3 + 1 // numbers, language and items length

	itemsRet := 0
	for _, item := range d.Items {
		itemsRet += 1 // description length
		itemsRet += len(item.Description)
		itemsRet += 1 // content length
		itemsRet += len(item.Content)
	}

	ret += itemsRet

	ret += 1 // text length
	ret += len(d.Text)

	return uint8(ret)
}

func (d *DescriptorExtendedEvent) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	var lengthOfItems int
	for _, item := range d.Items {
		lengthOfItems += 1 // description length
		lengthOfItems += len(item.Description)
		lengthOfItems += 1 // content length
		lengthOfItems += len(item.Content)
	}

	b.WriteN(d.Number, 4)
	b.WriteN(d.LastDescriptorNumber, 4)

	b.Write(d.ISO639LanguageCode[:])

	b.Write(uint8(lengthOfItems))
	for _, item := range d.Items {
		b.Write(uint8(len(item.Description)))
		b.Write(item.Description)
		b.Write(uint8(len(item.Content)))
		b.Write(item.Content)
	}

	b.Write(uint8(len(d.Text)))
	b.Write(d.Text)

	return written, b.Err()
}
