package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/internal/bytesiter"
)

// Audio types
// Page: 683 | https://books.google.fr/books?id=6dgWB3-rChYC&printsec=frontcover&hl=fr
const (
	AudioTypeCleanEffects             = 0x1
	AudioTypeHearingImpaired          = 0x2
	AudioTypeVisualImpairedCommentary = 0x3
)

// ISO639LanguageAndAudioType represents an ISO639 language descriptor
// https://github.com/gfto/bitstream/blob/master/mpeg/psi/desc_0a.h
// FIXME (barbashov) according to Chapter 2.6.18 ISO/IEC 13818-1:2015 there could be not one, but multiple such descriptors
type ISO639LanguageAndAudioType struct {
	Language [3]byte
	Header   Header
	Type     uint8
}

// In some actual cases, the length is 3 and the language is described in only 2 bytes
//

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
		Type:   bs[len(bs)-1],
	}
	copy(d.Language[:], bs)
	dd = d
	return
}

func (*ISO639LanguageAndAudioType) CalcLength() int {
	return 3 + 1 // language code + type
}

func (d *ISO639LanguageAndAudioType) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, d.Language[:]...)
	return append(dst, d.Type)
}
