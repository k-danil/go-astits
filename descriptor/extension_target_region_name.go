package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// ExtensionTargetRegionName represents a target region name extension
// descriptor: the names of target regions within a country, in one language.
// Chapter: 6.4.12 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type ExtensionTargetRegionName struct {
	Regions     []TargetRegionName
	CountryCode [3]byte
	Language    [3]byte
}

// TargetRegionName is one named region. SecondaryRegionCode is present for
// RegionDepth >= 2 and TertiaryRegionCode for RegionDepth == 3.
type TargetRegionName struct {
	RegionName          []byte
	TertiaryRegionCode  uint16
	RegionDepth         uint8
	PrimaryRegionCode   uint8
	SecondaryRegionCode uint8
}

func newDescriptorExtensionTargetRegionName(i *bytesiter.Iterator, offsetEnd int) (d *ExtensionTargetRegionName, err error) {
	d = &ExtensionTargetRegionName{}

	var bs []byte
	if bs, err = i.NextBytesNoCopy(6); err != nil || len(bs) < 6 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	copy(d.CountryCode[:], bs[0:3])
	copy(d.Language[:], bs[3:6])

	for i.Offset() < offsetEnd {
		var reg TargetRegionName
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

func (reg *TargetRegionName) length() int {
	n := 2 + len(reg.RegionName)
	if reg.RegionDepth >= 2 {
		n++
	}
	if reg.RegionDepth == 3 {
		n += 2
	}
	return n
}

func (d *ExtensionTargetRegionName) CalcLength() (n int) {
	n = 6
	for idx := range d.Regions {
		n += d.Regions[idx].length()
	}
	return
}

func (d *ExtensionTargetRegionName) Append(dst []byte) []byte {
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
