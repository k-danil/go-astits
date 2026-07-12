package ext

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// DTSHD represents a DTS-HD audio stream extension descriptor: the
// core and up to four extension substreams present, each described by a
// DTSHDSubstream. A nil substream pointer means that substream is absent.
// Chapter: G.3 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DTSHD struct {
	AdditionalInfo []byte
	CoreSubstream  *DTSHDSubstream
	Substream0     *DTSHDSubstream
	Substream1     *DTSHDSubstream
	Substream2     *DTSHDSubstream
	Substream3     *DTSHDSubstream
}

// DTSHDSubstream is one substream of a DTS-HD descriptor
type DTSHDSubstream struct {
	Assets            []DTSHDAsset
	ChannelCount      uint8
	SamplingFrequency uint8
	LFEFlag           bool
	SampleResolution  bool
}

// DTSHDAsset is one audio asset of a DTS-HD substream. BitRate carries either
// bit_rate or bit_rate_scaled per PostEncodeBRScalingFlag; ComponentType and
// Language are present per their flags.
type DTSHDAsset struct {
	Language                [3]byte
	BitRate                 uint16
	AssetConstruction       uint8
	ComponentType           uint8
	VBRFlag                 bool
	PostEncodeBRScalingFlag bool
	ComponentTypeFlag       bool
	LanguageCodeFlag        bool
}

func parseDTSHD(i *bytesiter.Iterator, offsetEnd int) (d *DTSHD, err error) {
	d = &DTSHD{}

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	for _, s := range []struct {
		present bool
		dst     **DTSHDSubstream
	}{
		{b&0x80 > 0, &d.CoreSubstream},
		{b&0x40 > 0, &d.Substream0},
		{b&0x20 > 0, &d.Substream1},
		{b&0x10 > 0, &d.Substream2},
		{b&0x08 > 0, &d.Substream3},
	} {
		if !s.present {
			continue
		}
		if *s.dst, err = parseDTSHDSubstream(i); err != nil {
			return
		}
	}

	if i.Offset() < offsetEnd {
		if d.AdditionalInfo, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func parseDTSHDSubstream(i *bytesiter.Iterator) (s *DTSHDSubstream, err error) {
	s = &DTSHDSubstream{}

	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	// bs[0] is substream_length, recomputed on Append.
	numAssets := int(bs[1] >> 5 & 0x07)
	s.ChannelCount = bs[1] & 0x1f
	s.LFEFlag = bs[2]&0x80 > 0
	s.SamplingFrequency = bs[2] >> 3 & 0x0f
	s.SampleResolution = bs[2]&0x04 > 0

	s.Assets = make([]DTSHDAsset, numAssets+1)
	for idx := range s.Assets {
		if err = parseDTSHDAsset(i, &s.Assets[idx]); err != nil {
			return
		}
	}
	return
}

func parseDTSHDAsset(i *bytesiter.Iterator, a *DTSHDAsset) (err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	w := uint32(bs[0])<<16 | uint32(bs[1])<<8 | uint32(bs[2])
	a.AssetConstruction = uint8(w >> 19 & 0x1f)
	a.VBRFlag = w&0x40000 > 0
	a.PostEncodeBRScalingFlag = w&0x20000 > 0
	a.ComponentTypeFlag = w&0x10000 > 0
	a.LanguageCodeFlag = w&0x8000 > 0
	a.BitRate = uint16(w >> 2 & 0x1fff)

	if a.ComponentTypeFlag {
		if a.ComponentType, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
	}
	if a.LanguageCodeFlag {
		if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		copy(a.Language[:], bs)
	}
	return
}

func (a *DTSHDAsset) length() int {
	n := 3
	if a.ComponentTypeFlag {
		n++
	}
	if a.LanguageCodeFlag {
		n += 3
	}
	return n
}

func (s *DTSHDSubstream) bodyLength() (n int) {
	n = 2
	for idx := range s.Assets {
		n += s.Assets[idx].length()
	}
	return
}

func (d *DTSHD) CalcLength() (n int) {
	n = 1
	for _, s := range []*DTSHDSubstream{d.CoreSubstream, d.Substream0, d.Substream1, d.Substream2, d.Substream3} {
		if s != nil {
			n += 1 + s.bodyLength()
		}
	}
	return n + len(d.AdditionalInfo)
}

func (d *DTSHD) Append(dst []byte) []byte {
	var b byte
	for _, s := range []struct {
		bit byte
		sub *DTSHDSubstream
	}{
		{0x80, d.CoreSubstream}, {0x40, d.Substream0}, {0x20, d.Substream1},
		{0x10, d.Substream2}, {0x08, d.Substream3},
	} {
		if s.sub != nil {
			b |= s.bit
		}
	}
	dst = append(dst, b)

	for _, s := range []*DTSHDSubstream{d.CoreSubstream, d.Substream0, d.Substream1, d.Substream2, d.Substream3} {
		if s != nil {
			dst = s.append(dst)
		}
	}
	return append(dst, d.AdditionalInfo...)
}

func (s *DTSHDSubstream) append(dst []byte) []byte {
	numAssets := uint8(len(s.Assets) - 1)
	dst = append(dst, uint8(s.bodyLength()), numAssets&0x07<<5|s.ChannelCount&0x1f)
	b := s.SamplingFrequency & 0x0f << 3
	if s.LFEFlag {
		b |= 0x80
	}
	if s.SampleResolution {
		b |= 0x04
	}
	dst = append(dst, b)
	for idx := range s.Assets {
		dst = s.Assets[idx].append(dst)
	}
	return dst
}

func (a *DTSHDAsset) append(dst []byte) []byte {
	w := uint32(a.AssetConstruction&0x1f)<<19 | uint32(a.BitRate&0x1fff)<<2
	if a.VBRFlag {
		w |= 0x40000
	}
	if a.PostEncodeBRScalingFlag {
		w |= 0x20000
	}
	if a.ComponentTypeFlag {
		w |= 0x10000
	}
	if a.LanguageCodeFlag {
		w |= 0x8000
	}
	dst = append(dst, byte(w>>16), byte(w>>8), byte(w))
	if a.ComponentTypeFlag {
		dst = append(dst, a.ComponentType)
	}
	if a.LanguageCodeFlag {
		dst = append(dst, a.Language[:]...)
	}
	return dst
}
