package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/dvbtext"
	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type MultilingualComponent struct {
	Items        []MultilingualComponentItem `json:"_items"`
	Header       Header                      `json:"_header"`
	ComponentTag uint8                       `json:"component_tag"`
}

type MultilingualComponentItem struct {
	Description dvbtext.Text `json:"text_char"`
	Language    dvbtext.Code `json:"ISO_639_language_code"`
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
		if err = readLangText(i, &item.Language, &item.Description); err != nil {
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
