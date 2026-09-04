package descriptor

import (
	"encoding/json"
	"fmt"

	"github.com/k-danil/go-astits/v3/dvbtext"
	"github.com/k-danil/go-astits/v3/internal/bytesiter"
	"github.com/k-danil/go-astits/v3/internal/util"
	"github.com/k-danil/go-astits/v3/ts"
)

type AudioType uint8

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

type ISO639LanguageAndAudioType struct {
	Header Header       `json:"_header"`
	Items  []ISO639Item `json:"_items"`
}

type ISO639Item struct {
	Language dvbtext.Code `json:"ISO_639_language_code"`
	Type     AudioType    `json:"audio_type"`
}

const iso639ItemLen = 4

func newDescriptorISO639LanguageAndAudioType(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(offsetEnd - i.Offset()); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	if len(bs)%iso639ItemLen != 0 {
		err = fmt.Errorf("astits: ISO 639 descriptor length %d is not a multiple of %d: %w", len(bs), iso639ItemLen, ts.ErrInvalidData)
		return
	}

	d := &ISO639LanguageAndAudioType{
		Header: h,
		Items:  make([]ISO639Item, 0, len(bs)/iso639ItemLen),
	}
	dd = d

	for len(bs) >= iso639ItemLen {
		var it ISO639Item
		copy(it.Language[:], bs[:3])
		it.Type = AudioType(bs[3])
		d.Items = append(d.Items, it)
		bs = bs[iso639ItemLen:]
	}
	return
}

func (d *ISO639LanguageAndAudioType) CalcLength() int {
	return iso639ItemLen * len(d.Items)
}

func (d *ISO639LanguageAndAudioType) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	for _, it := range d.Items {
		dst = append(dst, it.Language[:]...)
		dst = append(dst, uint8(it.Type))
	}
	return dst
}
