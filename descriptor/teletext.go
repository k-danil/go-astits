package descriptor

import (
	"encoding/json"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

type TeletextType uint8

// Teletext types
// Chapter: 6.2.43 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
const (
	TeletextTypeAdditionalInformationPage                    TeletextType = 0x3
	TeletextTypeInitialTeletextPage                          TeletextType = 0x1
	TeletextTypeProgramSchedulePage                          TeletextType = 0x4
	TeletextTypeTeletextSubtitlePage                         TeletextType = 0x2
	TeletextTypeTeletextSubtitlePageForHearingImpairedPeople TeletextType = 0x5
)

var teletextTypeNames = map[TeletextType]string{
	TeletextTypeInitialTeletextPage:                          "initial_Teletext_page",
	TeletextTypeTeletextSubtitlePage:                         "Teletext_subtitle_page",
	TeletextTypeAdditionalInformationPage:                    "additional_information_page",
	TeletextTypeProgramSchedulePage:                          "programme_schedule_page",
	TeletextTypeTeletextSubtitlePageForHearingImpairedPeople: "Teletext_subtitle_page_for_hearing_impaired_people",
}

func (t TeletextType) String() (s string) {
	var ok bool
	if s, ok = teletextTypeNames[t]; !ok {
		s = fmt.Sprintf("0x%02x", uint8(t))
	}
	return
}

func (t TeletextType) MarshalJSON() (b []byte, err error) {
	return json.Marshal(t.String())
}

func (t *TeletextType) UnmarshalJSON(b []byte) (err error) {
	*t, err = util.UnmarshalEnum(b, teletextTypeNames)
	return
}

// Teletext represents a teletext descriptor
// Chapter: 6.2.43 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type Teletext struct {
	Header Header         `json:"_header"`
	Items  []TeletextItem `json:"_items"`
}

// TeletextItem represents a teletext descriptor item
// Chapter: 6.2.43 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type TeletextItem struct {
	Language [3]byte      `json:"ISO_639_language_code"`
	Magazine uint8        `json:"teletext_magazine_number"`
	Page     uint8        `json:"teletext_page_number"`
	Type     TeletextType `json:"teletext_type"`
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

		d.Items[idx].Type = TeletextType(b >> 3)

		d.Items[idx].Magazine = b & 0x7

		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		d.Items[idx].Page = b
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
		dst = append(dst, uint8(item.Type)&0x1f<<3|item.Magazine&0x7)
		dst = append(dst, item.Page)
	}
	return dst
}
