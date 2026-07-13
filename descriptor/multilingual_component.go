package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// MultilingualComponent represents a multilingual component descriptor: a text
// description of a component (identified by ComponentTag) in one or more
// languages.
// Chapter: 6.2.23 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type MultilingualComponent struct {
	Items        []MultilingualComponentItem `json:"_items"`
	Header       Header                      `json:"_header"`
	ComponentTag uint8                       `json:"component_tag"`
}

// MultilingualComponentItem is one language variant of a component description
type MultilingualComponentItem struct {
	Description []byte  `json:"text_char"`
	Language    [3]byte `json:"ISO_639_language_code"`
}

func newDescriptorMultilingualComponent(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &MultilingualComponent{
		Header: h,
	}
	dd = d

	if d.ComponentTag, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	for i.Offset() < offsetEnd {
		var item MultilingualComponentItem
		if err = readLangText(i, item.Language[:], &item.Description); err != nil {
			return
		}
		d.Items = append(d.Items, item)
	}
	return
}

func (d *MultilingualComponent) CalcLength() (n int) {
	n = 1
	for _, item := range d.Items {
		n += 4 + len(item.Description)
	}
	return
}

func (d *MultilingualComponent) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()), d.ComponentTag)
	for _, item := range d.Items {
		dst = append(dst, item.Language[:]...)
		dst = append(dst, uint8(len(item.Description)))
		dst = append(dst, item.Description...)
	}
	return dst
}
