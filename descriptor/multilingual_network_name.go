package descriptor

import (
	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// MultilingualNetworkName represents a multilingual network name descriptor:
// the network name in text form in one or more languages.
// Chapter: 6.2.24 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type MultilingualNetworkName struct {
	Items  []MultilingualNetworkNameItem
	Header Header
}

// MultilingualNetworkNameItem is one language variant of a network name
type MultilingualNetworkNameItem struct {
	Name     []byte
	Language [3]byte
}

func newDescriptorMultilingualNetworkName(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &MultilingualNetworkName{
		Header: h,
	}
	dd = d

	for i.Offset() < offsetEnd {
		var item MultilingualNetworkNameItem
		if err = readLangText(i, item.Language[:], &item.Name); err != nil {
			return
		}
		d.Items = append(d.Items, item)
	}
	return
}

func (d *MultilingualNetworkName) CalcLength() (n int) {
	for _, item := range d.Items {
		n += 4 + len(item.Name)
	}
	return
}

func (d *MultilingualNetworkName) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	for _, item := range d.Items {
		dst = append(dst, item.Language[:]...)
		dst = append(dst, uint8(len(item.Name)))
		dst = append(dst, item.Name...)
	}
	return dst
}
