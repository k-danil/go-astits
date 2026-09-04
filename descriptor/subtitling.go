package descriptor

import (
	"encoding/binary"
	"fmt"
	"github.com/k-danil/go-astits/v3/dvbtext"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type Subtitling struct {
	Header Header           `json:"_header"`
	Items  []SubtitlingItem `json:"_items"`
}

type SubtitlingItem struct {
	AncillaryPageID   uint16       `json:"ancillary_page_id"`
	CompositionPageID uint16       `json:"composition_page_id"`
	Language          dvbtext.Code `json:"ISO_639_language_code"`
	Type              uint8        `json:"subtitling_type"`
}

func newDescriptorSubtitling(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &Subtitling{
		Header: h,
		Items:  make([]SubtitlingItem, (offsetEnd-i.Offset())/8),
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

		d.Items[idx].Type = b

		if bs, err = i.NextBytesNoCopy(2); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}

		d.Items[idx].CompositionPageID = binary.BigEndian.Uint16(bs)

		if bs, err = i.NextBytesNoCopy(2); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}

		d.Items[idx].AncillaryPageID = binary.BigEndian.Uint16(bs)
	}

	err = rejectTrailingBytes(i, offsetEnd)
	return
}

func (d *Subtitling) CalcLength() int {
	return 8 * len(d.Items)
}

func (d *Subtitling) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	for _, item := range d.Items {
		dst = append(dst, item.Language[:]...)
		dst = append(dst, item.Type,
			byte(item.CompositionPageID>>8), byte(item.CompositionPageID),
			byte(item.AncillaryPageID>>8), byte(item.AncillaryPageID))
	}
	return dst
}
