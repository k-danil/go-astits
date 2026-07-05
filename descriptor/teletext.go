package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
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

// Teletext represents a teletext descriptor
// Chapter: 6.2.43 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type Teletext struct {
	Header Header
	Items  []TeletextItem
}

// TeletextItem represents a teletext descriptor item
// Chapter: 6.2.43 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type TeletextItem struct {
	Language [3]byte
	Magazine uint8
	Page     uint8
	Type     uint8
}

func newDescriptorTeletext(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &Teletext{
		Header: h,
		Items:  make([]TeletextItem, (offsetEnd-i.Offset())/5),
	}
	dd = d

	for idx := range d.Items {
		var bs []byte
		if bs, err = i.NextBytesNoCopy(3); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		copy(d.Items[idx].Language[:], bs)

		var b byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		d.Items[idx].Type = b >> 3

		d.Items[idx].Magazine = b & 0x7

		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		d.Items[idx].Page = b>>4*10 + b&0xf
	}
	return
}

func (d *Teletext) CalcLength() int {
	return 5 * len(d.Items)
}

func (d *Teletext) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	for _, item := range d.Items {
		dst = append(dst, item.Language[:]...)
		dst = append(dst, item.Type&0x1f<<3|item.Magazine&0x7)
		dst = append(dst, item.Page/10<<4|item.Page%10&0xf)
	}
	return dst
}
