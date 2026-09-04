package descriptor

import (
	"fmt"
	"github.com/k-danil/go-astits/v3/dvbtext"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type ParentalRating struct {
	Header Header               `json:"_header"`
	Items  []ParentalRatingItem `json:"_items"`
}

type ParentalRatingItem struct {
	CountryCode dvbtext.Code `json:"country_code"`
	Rating      uint8        `json:"rating"`
}

const (
	ratingUndefined          = 0x00
	ratingBroadcasterDefined = 0x10
	ratingAgeOffset          = 3
)

// MinimumAge returns 0 when the rating is undefined or broadcaster-defined.
func (d ParentalRatingItem) MinimumAge() int {
	if d.Rating == ratingUndefined || d.Rating >= ratingBroadcasterDefined {
		return 0
	}
	return int(d.Rating) + ratingAgeOffset
}

func newDescriptorParentalRating(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &ParentalRating{
		Header: h,
		Items:  make([]ParentalRatingItem, (offsetEnd-i.Offset())/4),
	}
	dd = d

	for idx := range d.Items {
		var bs []byte
		if bs, err = i.NextBytesNoCopy(4); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		d.Items[idx].Rating = bs[3]
		copy(d.Items[idx].CountryCode[:], bs)
	}

	err = rejectTrailingBytes(i, offsetEnd)
	return
}

func (d *ParentalRating) CalcLength() int {
	return 4 * len(d.Items)
}

func (d *ParentalRating) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	for _, item := range d.Items {
		dst = append(dst, item.CountryCode[:]...)
		dst = append(dst, item.Rating)
	}
	return dst
}
