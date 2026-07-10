package descriptor

import (
	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// MultilingualBouquetName represents a multilingual bouquet name descriptor:
// the bouquet name in text form in one or more languages.
// Chapter: 6.2.22 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type MultilingualBouquetName struct {
	Items  []MultilingualBouquetNameItem
	Header Header
}

// MultilingualBouquetNameItem is one language variant of a bouquet name
type MultilingualBouquetNameItem struct {
	Name     []byte
	Language [3]byte
}

func newDescriptorMultilingualBouquetName(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &MultilingualBouquetName{
		Header: h,
	}
	dd = d

	for i.Offset() < offsetEnd {
		var item MultilingualBouquetNameItem
		if err = readLangText(i, item.Language[:], &item.Name); err != nil {
			return
		}
		d.Items = append(d.Items, item)
	}
	return
}

func (d *MultilingualBouquetName) CalcLength() (n int) {
	for _, item := range d.Items {
		n += 4 + len(item.Name)
	}
	return
}

func (d *MultilingualBouquetName) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	for _, item := range d.Items {
		dst = append(dst, item.Language[:]...)
		dst = append(dst, uint8(len(item.Name)))
		dst = append(dst, item.Name...)
	}
	return dst
}
