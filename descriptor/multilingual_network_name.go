package descriptor

import (
	"github.com/k-danil/go-astits/v3/dvbtext"
	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type MultilingualNetworkName struct {
	Items  []MultilingualNetworkNameItem `json:"_items"`
	Header Header                        `json:"_header"`
}

type MultilingualNetworkNameItem struct {
	Name     dvbtext.Text `json:"network_name"`
	Language dvbtext.Code `json:"ISO_639_language_code"`
}

func newDescriptorMultilingualNetworkName(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &MultilingualNetworkName{
		Header: h,
	}
	dd = d

	for i.Offset() < offsetEnd {
		var item MultilingualNetworkNameItem
		if err = readLangText(i, &item.Language, &item.Name); err != nil {
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
