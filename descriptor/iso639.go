package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// Audio types
// Page: 683 | https://books.google.fr/books?id=6dgWB3-rChYC&printsec=frontcover&hl=fr
const (
	AudioTypeCleanEffects             = 0x1
	AudioTypeHearingImpaired          = 0x2
	AudioTypeVisualImpairedCommentary = 0x3
)

// ISO639LanguageAndAudioType represents an ISO639 language descriptor:
// a list of language+audio-type entries (Chapter 2.6.18 ISO/IEC 13818-1:2015)
type ISO639LanguageAndAudioType struct {
	Header Header
	Items  []ISO639Item
}

type ISO639Item struct {
	Language [3]byte
	Type     uint8
}

func newDescriptorISO639LanguageAndAudioType(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	// Get next bytes
	var bs []byte
	if bs, err = i.NextBytesNoCopy(offsetEnd - i.Offset()); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	// Create descriptor
	d := &ISO639LanguageAndAudioType{
		Header: h,
		Items:  make([]ISO639Item, 0, len(bs)/4),
	}
	dd = d

	for len(bs) >= 4 {
		var it ISO639Item
		copy(it.Language[:], bs[:3])
		it.Type = bs[3]
		d.Items = append(d.Items, it)
		bs = bs[4:]
	}
	// Degenerate entries happen in the wild: length 3 with a 2-byte language
	if len(bs) > 0 {
		var it ISO639Item
		copy(it.Language[:], bs[:len(bs)-1])
		it.Type = bs[len(bs)-1]
		d.Items = append(d.Items, it)
	}
	return
}

func (d *ISO639LanguageAndAudioType) CalcLength() int {
	return 4 * len(d.Items) // language code + type each
}

func (d *ISO639LanguageAndAudioType) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	for _, it := range d.Items {
		dst = append(dst, it.Language[:]...)
		dst = append(dst, it.Type)
	}
	return dst
}
