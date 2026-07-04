package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// ParentalRating represents a parental rating descriptor
// Chapter: 6.2.28 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type ParentalRating struct {
	Header Header
	Items  []ParentalRatingItem
}

// ParentalRatingItem represents a parental rating item descriptor
// Chapter: 6.2.28 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type ParentalRatingItem struct {
	CountryCode [3]byte
	Rating      uint8
}

// MinimumAge returns the minimum age for the parental rating
func (d ParentalRatingItem) MinimumAge() int {
	// Undefined or user defined ratings
	if d.Rating == 0 || d.Rating > 0x10 {
		return 0
	}
	return int(d.Rating) + 3
}

func newDescriptorParentalRating(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	// Create descriptor
	d := &ParentalRating{
		Header: h,
		Items:  make([]ParentalRatingItem, (offsetEnd-i.Offset())/4),
	}
	dd = d

	// Add items
	for idx := range d.Items {
		// Get next bytes
		var bs []byte
		if bs, err = i.NextBytesNoCopy(4); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		d.Items[idx].Rating = bs[3]
		copy(d.Items[idx].CountryCode[:], bs)
	}
	return
}

func (d *ParentalRating) CalcLength() int {
	return 4 * len(d.Items)
}

func (d *ParentalRating) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	for _, item := range d.Items {
		dst = append(dst, item.CountryCode[:]...)
		dst = append(dst, item.Rating)
	}
	return dst
}
