package ext

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// TargetRegionName represents a target region name extension
// descriptor: the names of target regions within a country, in one language.
// Chapter: 6.4.12 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type TargetRegionName struct {
	Regions     []NamedRegion `json:"_regions"`
	CountryCode [3]byte       `json:"country_code"`
	Language    [3]byte       `json:"ISO_639_language_code"`
}

// NamedRegion is one named region. SecondaryRegionCode is present for
// RegionDepth >= 2 and TertiaryRegionCode for RegionDepth == 3.
type NamedRegion struct {
	RegionName          []byte `json:"region_name"`
	TertiaryRegionCode  uint16 `json:"tertiary_region_code"`
	RegionDepth         uint8  `json:"region_depth"`
	PrimaryRegionCode   uint8  `json:"primary_region_code"`
	SecondaryRegionCode uint8  `json:"secondary_region_code"`
}

func parseTargetRegionName(i *bytesiter.Iterator, offsetEnd int) (d *TargetRegionName, err error) {
	d = &TargetRegionName{}

	var bs []byte
	if bs, err = i.NextBytesNoCopy(6); err != nil || len(bs) < 6 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	copy(d.CountryCode[:], bs[0:3])
	copy(d.Language[:], bs[3:6])

	for i.Offset() < offsetEnd {
		var reg NamedRegion
		var b byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		reg.RegionDepth = b >> 6 & 0x03
		if reg.RegionName, err = i.NextBytes(int(b & 0x3f)); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		if reg.PrimaryRegionCode, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		if reg.RegionDepth >= 2 {
			if reg.SecondaryRegionCode, err = i.NextByte(); err != nil {
				err = fmt.Errorf("astits: fetching next byte failed: %w", err)
				return
			}
		}
		if reg.RegionDepth == 3 {
			if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
				err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
				return
			}
			reg.TertiaryRegionCode = binary.BigEndian.Uint16(bs)
		}
		d.Regions = append(d.Regions, reg)
	}
	return
}

func (reg *NamedRegion) length() int {
	n := 2 + len(reg.RegionName)
	if reg.RegionDepth >= 2 {
		n++
	}
	if reg.RegionDepth == 3 {
		n += 2
	}
	return n
}

func (d *TargetRegionName) CalcLength() (n int) {
	n = 6
	for idx := range d.Regions {
		n += d.Regions[idx].length()
	}
	return
}

func (d *TargetRegionName) Append(dst []byte) []byte {
	dst = append(dst, d.CountryCode[:]...)
	dst = append(dst, d.Language[:]...)
	for idx := range d.Regions {
		reg := &d.Regions[idx]
		dst = append(dst, reg.RegionDepth&0x03<<6|uint8(len(reg.RegionName))&0x3f)
		dst = append(dst, reg.RegionName...)
		dst = append(dst, reg.PrimaryRegionCode)
		if reg.RegionDepth >= 2 {
			dst = append(dst, reg.SecondaryRegionCode)
		}
		if reg.RegionDepth == 3 {
			dst = append(dst, byte(reg.TertiaryRegionCode>>8), byte(reg.TertiaryRegionCode))
		}
	}
	return dst
}
