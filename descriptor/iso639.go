package descriptor

import (
	"fmt"

	"github.com/asticode/go-astikit"
)

// Audio types
// Page: 683 | https://books.google.fr/books?id=6dgWB3-rChYC&printsec=frontcover&hl=fr
const (
	AudioTypeCleanEffects             = 0x1
	AudioTypeHearingImpaired          = 0x2
	AudioTypeVisualImpairedCommentary = 0x3
)

// DescriptorISO639LanguageAndAudioType represents an ISO639 language descriptor
// https://github.com/gfto/bitstream/blob/master/mpeg/psi/desc_0a.h
// FIXME (barbashov) according to Chapter 2.6.18 ISO/IEC 13818-1:2015 there could be not one, but multiple such descriptors
type DescriptorISO639LanguageAndAudioType struct {
	Language [3]byte
	Header   DescriptorHeader
	Type     uint8
}

// In some actual cases, the length is 3 and the language is described in only 2 bytes
//

func newDescriptorISO639LanguageAndAudioType(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (dd Descriptor, err error) {
	// Get next bytes
	var bs []byte
	if bs, err = i.NextBytesNoCopy(offsetEnd - i.Offset()); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	// Create descriptor
	d := &DescriptorISO639LanguageAndAudioType{
		Header: h,
		Type:   bs[len(bs)-1],
	}
	copy(d.Language[:], bs)
	dd = d
	return
}

func (*DescriptorISO639LanguageAndAudioType) length() uint8 {
	return 3 + 1 // language code + type
}

func (d *DescriptorISO639LanguageAndAudioType) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.Language[:])
	b.Write(d.Type)

	return written, b.Err()
}
