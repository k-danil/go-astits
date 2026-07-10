package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// MultilingualServiceName represents a multilingual service name descriptor:
// the service provider and service names in one or more languages.
// Chapter: 6.2.25 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type MultilingualServiceName struct {
	Items  []MultilingualServiceNameItem
	Header Header
}

// MultilingualServiceNameItem is one language variant of a service name
type MultilingualServiceNameItem struct {
	Provider []byte
	Name     []byte
	Language [3]byte
}

func newDescriptorMultilingualServiceName(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &MultilingualServiceName{
		Header: h,
	}
	dd = d

	for i.Offset() < offsetEnd {
		var item MultilingualServiceNameItem
		if err = readLangText(i, item.Language[:], &item.Provider); err != nil {
			return
		}
		var b byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		if item.Name, err = i.NextBytes(int(b)); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		d.Items = append(d.Items, item)
	}
	return
}

func (d *MultilingualServiceName) CalcLength() (n int) {
	for _, item := range d.Items {
		n += 5 + len(item.Provider) + len(item.Name)
	}
	return
}

func (d *MultilingualServiceName) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	for _, item := range d.Items {
		dst = append(dst, item.Language[:]...)
		dst = append(dst, uint8(len(item.Provider)))
		dst = append(dst, item.Provider...)
		dst = append(dst, uint8(len(item.Name)))
		dst = append(dst, item.Name...)
	}
	return dst
}
