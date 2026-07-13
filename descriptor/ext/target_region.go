package ext

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// TargetRegion represents a target region extension descriptor: a set
// of target regions, each qualified by a country code and a region depth.
// Chapter: 6.4.11 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type TargetRegion struct {
	Regions     []Region `json:"_regions"`
	CountryCode [3]byte  `json:"country_code"`
}

// Region is one region loop of a target region descriptor. Which region
// codes are present is selected by RegionDepth (1: primary; 2: +secondary;
// 3: +tertiary); CountryCode is present only when CountryCodeFlag.
type Region struct {
	CountryCode         [3]byte `json:"country_code"`
	TertiaryRegionCode  uint16  `json:"tertiary_region_code"`
	RegionDepth         uint8   `json:"region_depth"`
	PrimaryRegionCode   uint8   `json:"primary_region_code"`
	SecondaryRegionCode uint8   `json:"secondary_region_code"`
	CountryCodeFlag     bool    `json:"country_code_flag"`
}

func parseTargetRegion(i *bytesiter.Iterator, offsetEnd int) (d *TargetRegion, err error) {
	d = &TargetRegion{}

	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	copy(d.CountryCode[:], bs)

	for i.Offset() < offsetEnd {
		var reg Region
		var b byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		reg.CountryCodeFlag = b&0x04 > 0
		reg.RegionDepth = b & 0x03

		if reg.CountryCodeFlag {
			if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
				err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
				return
			}
			copy(reg.CountryCode[:], bs)
		}
		if reg.RegionDepth >= 1 {
			if reg.PrimaryRegionCode, err = i.NextByte(); err != nil {
				err = fmt.Errorf("astits: fetching next byte failed: %w", err)
				return
			}
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

func (reg *Region) length() int {
	n := 1
	if reg.CountryCodeFlag {
		n += 3
	}
	if reg.RegionDepth >= 1 {
		n++
	}
	if reg.RegionDepth >= 2 {
		n++
	}
	if reg.RegionDepth == 3 {
		n += 2
	}
	return n
}

func (d *TargetRegion) CalcLength() (n int) {
	n = 3
	for idx := range d.Regions {
		n += d.Regions[idx].length()
	}
	return
}

func (d *TargetRegion) Append(dst []byte) []byte {
	dst = append(dst, d.CountryCode[:]...)
	for idx := range d.Regions {
		reg := &d.Regions[idx]
		b := byte(0xf8) | reg.RegionDepth&0x03
		if reg.CountryCodeFlag {
			b |= 0x04
		}
		dst = append(dst, b)
		if reg.CountryCodeFlag {
			dst = append(dst, reg.CountryCode[:]...)
		}
		if reg.RegionDepth >= 1 {
			dst = append(dst, reg.PrimaryRegionCode)
		}
		if reg.RegionDepth >= 2 {
			dst = append(dst, reg.SecondaryRegionCode)
		}
		if reg.RegionDepth == 3 {
			dst = append(dst, byte(reg.TertiaryRegionCode>>8), byte(reg.TertiaryRegionCode))
		}
	}
	return dst
}
