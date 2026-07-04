package descriptor

import (
	"fmt"

	"github.com/asticode/go-astikit"
)

// DescriptorParentalRating represents a parental rating descriptor
// Chapter: 6.2.28 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorParentalRating struct {
	Header DescriptorHeader
	Items  []DescriptorParentalRatingItem
}

// DescriptorParentalRatingItem represents a parental rating item descriptor
// Chapter: 6.2.28 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorParentalRatingItem struct {
	CountryCode [3]byte
	Rating      uint8
}

// MinimumAge returns the minimum age for the parental rating
func (d DescriptorParentalRatingItem) MinimumAge() int {
	// Undefined or user defined ratings
	if d.Rating == 0 || d.Rating > 0x10 {
		return 0
	}
	return int(d.Rating) + 3
}

func newDescriptorParentalRating(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (dd Descriptor, err error) {
	// Create descriptor
	d := &DescriptorParentalRating{
		Header: h,
		Items:  make([]DescriptorParentalRatingItem, (offsetEnd-i.Offset())/4),
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

func (d *DescriptorParentalRating) length() uint8 {
	return uint8(4 * len(d.Items))
}

func (d *DescriptorParentalRating) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	for _, item := range d.Items {
		b.Write(item.CountryCode[:])
		b.Write(item.Rating)
	}

	return written, b.Err()
}
