package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// ExtendedEvent represents an extended event descriptor
// Chapter: 6.2.15 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type ExtendedEvent struct {
	Text                 []byte              `json:"text_char"`
	Items                []ExtendedEventItem `json:"_items"`
	Header               Header              `json:"_header"`
	ISO639LanguageCode   [3]byte             `json:"ISO_639_language_code"`
	LastDescriptorNumber uint8               `json:"last_descriptor_number"`
	Number               uint8               `json:"descriptor_number"`
}

// ExtendedEventItem represents an extended event item descriptor
// Chapter: 6.2.15 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type ExtendedEventItem struct {
	Content     []byte `json:"item_char"`
	Description []byte `json:"item_description"`
}

func newDescriptorExtendedEvent(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	d := &ExtendedEvent{
		Header: h,
	}
	dd = d

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	d.Number = b >> 4

	d.LastDescriptorNumber = b & 0xf

	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	copy(d.ISO639LanguageCode[:], bs)

	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	itemsLength := int(b)

	offsetEnd := i.Offset() + itemsLength
	for i.Offset() < offsetEnd {
		var item ExtendedEventItem
		if err = item.newDescriptorExtendedEventItem(i); err != nil {
			err = fmt.Errorf("astits: creating extended event item failed: %w", err)
			return
		}

		d.Items = append(d.Items, item)
	}

	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	textLength := int(b)

	if d.Text, err = i.NextBytes(textLength); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *ExtendedEventItem) newDescriptorExtendedEventItem(i *bytesiter.Iterator) (err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	descriptionLength := int(b)

	if d.Description, err = i.NextBytes(descriptionLength); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	contentLength := int(b)

	if d.Content, err = i.NextBytes(contentLength); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *ExtendedEvent) CalcLength() int {
	ret := 1 + 3 + 1 // numbers, language and items length

	for _, item := range d.Items {
		ret += 1 // description length
		ret += len(item.Description)
		ret += 1 // content length
		ret += len(item.Content)
	}

	ret += 1 // text length
	ret += len(d.Text)

	return ret
}

func (d *ExtendedEvent) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	var lengthOfItems int
	for _, item := range d.Items {
		lengthOfItems += 1 // description length
		lengthOfItems += len(item.Description)
		lengthOfItems += 1 // content length
		lengthOfItems += len(item.Content)
	}

	dst = append(dst, d.Number&0xf<<4|d.LastDescriptorNumber&0xf)
	dst = append(dst, d.ISO639LanguageCode[:]...)

	dst = append(dst, uint8(lengthOfItems))
	for _, item := range d.Items {
		dst = append(dst, uint8(len(item.Description)))
		dst = append(dst, item.Description...)
		dst = append(dst, uint8(len(item.Content)))
		dst = append(dst, item.Content...)
	}

	dst = append(dst, uint8(len(d.Text)))
	return append(dst, d.Text...)
}
