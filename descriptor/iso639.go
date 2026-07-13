package descriptor

import (
	"encoding/json"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

type AudioType uint8

// Audio types
// Page: 683 | https://books.google.fr/books?id=6dgWB3-rChYC&printsec=frontcover&hl=fr
const (
	AudioTypeUndefined                AudioType = 0x0
	AudioTypeCleanEffects             AudioType = 0x1
	AudioTypeHearingImpaired          AudioType = 0x2
	AudioTypeVisualImpairedCommentary AudioType = 0x3
)

var audioTypeNames = map[AudioType]string{
	AudioTypeUndefined:                "undefined",
	AudioTypeCleanEffects:             "clean_effects",
	AudioTypeHearingImpaired:          "hearing_impaired",
	AudioTypeVisualImpairedCommentary: "visual_impaired_commentary",
}

func (t AudioType) String() (s string) {
	var ok bool
	if s, ok = audioTypeNames[t]; !ok {
		s = fmt.Sprintf("0x%02x", uint8(t))
	}
	return
}

func (t AudioType) MarshalJSON() (b []byte, err error) {
	return json.Marshal(t.String())
}

func (t *AudioType) UnmarshalJSON(b []byte) (err error) {
	*t, err = util.UnmarshalEnum(b, audioTypeNames)
	return
}

// ISO639LanguageAndAudioType represents an ISO639 language descriptor:
// a list of language+audio-type entries (Chapter 2.6.18 ISO/IEC 13818-1:2015)
type ISO639LanguageAndAudioType struct {
	Header Header       `json:"_header"`
	Items  []ISO639Item `json:"_items"`
}

// ISO639Item is one language + audio-type entry of an ISO 639 descriptor.
type ISO639Item struct {
	Language [3]byte   `json:"ISO_639_language_code"`
	Type     AudioType `json:"audio_type"`
}

func newDescriptorISO639LanguageAndAudioType(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(offsetEnd - i.Offset()); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &ISO639LanguageAndAudioType{
		Header: h,
		Items:  make([]ISO639Item, 0, len(bs)/4),
	}
	dd = d

	for len(bs) >= 4 {
		var it ISO639Item
		copy(it.Language[:], bs[:3])
		it.Type = AudioType(bs[3])
		d.Items = append(d.Items, it)
		bs = bs[4:]
	}
	// Degenerate entries happen in the wild: length 3 with a 2-byte language
	if len(bs) > 0 {
		var it ISO639Item
		copy(it.Language[:], bs[:len(bs)-1])
		it.Type = AudioType(bs[len(bs)-1])
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
		dst = append(dst, uint8(it.Type))
	}
	return dst
}
