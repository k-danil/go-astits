package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// Content represents a content descriptor
// Chapter: 6.2.9 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type Content struct {
	Header Header
	Items  []ContentItem
}

// ContentItem represents a content item descriptor
// Chapter: 6.2.9 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type ContentItem struct {
	ContentNibbleLevel1 uint8
	ContentNibbleLevel2 uint8
	UserByte            uint8
}

func newDescriptorContent(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &Content{
		Header: h,
	}
	dd = d

	for i.Offset() < offsetEnd {
		var bs []byte
		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}

		d.Items = append(d.Items, ContentItem{
			ContentNibbleLevel1: bs[0] >> 4,
			ContentNibbleLevel2: bs[0] & 0xf,
			UserByte:            bs[1],
		})
	}
	return
}

func (d *Content) CalcLength() int {
	return 2 * len(d.Items)
}

func (d *Content) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	for _, item := range d.Items {
		dst = append(dst, item.ContentNibbleLevel1<<4|item.ContentNibbleLevel2&0xf, item.UserByte)
	}
	return dst
}
