package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// Telephone represents a telephone descriptor: a telephone number (split into
// its prefix/area/operator/national/core parts) for narrowband interactive
// channels.
// Chapter: 6.2.42 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type Telephone struct {
	CountryPrefix         []byte
	InternationalAreaCode []byte
	OperatorCode          []byte
	NationalAreaCode      []byte
	CoreNumber            []byte
	Header                Header
	ConnectionType        uint8
	ForeignAvailability   bool
}

func newDescriptorTelephone(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	d := &Telephone{
		Header: h,
	}
	dd = d

	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d.ForeignAvailability = bs[0]&0x20 > 0
	d.ConnectionType = bs[0] & 0x1f
	countryPrefixLength := int(bs[1] >> 5 & 0x03)
	internationalAreaCodeLength := int(bs[1] >> 2 & 0x07)
	operatorCodeLength := int(bs[1] & 0x03)
	nationalAreaCodeLength := int(bs[2] >> 4 & 0x07)
	coreNumberLength := int(bs[2] & 0x0f)

	for _, part := range []struct {
		dst    *[]byte
		length int
	}{
		{&d.CountryPrefix, countryPrefixLength},
		{&d.InternationalAreaCode, internationalAreaCodeLength},
		{&d.OperatorCode, operatorCodeLength},
		{&d.NationalAreaCode, nationalAreaCodeLength},
		{&d.CoreNumber, coreNumberLength},
	} {
		if *part.dst, err = i.NextBytes(part.length); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *Telephone) CalcLength() int {
	return 3 + len(d.CountryPrefix) + len(d.InternationalAreaCode) +
		len(d.OperatorCode) + len(d.NationalAreaCode) + len(d.CoreNumber)
}

func (d *Telephone) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	b0 := byte(0xc0) | d.ConnectionType&0x1f
	if d.ForeignAvailability {
		b0 |= 0x20
	}
	b1 := byte(0x80) | uint8(len(d.CountryPrefix))&0x03<<5 |
		uint8(len(d.InternationalAreaCode))&0x07<<2 | uint8(len(d.OperatorCode))&0x03
	b2 := byte(0x80) | uint8(len(d.NationalAreaCode))&0x07<<4 | uint8(len(d.CoreNumber))&0x0f
	dst = append(dst, b0, b1, b2)
	dst = append(dst, d.CountryPrefix...)
	dst = append(dst, d.InternationalAreaCode...)
	dst = append(dst, d.OperatorCode...)
	dst = append(dst, d.NationalAreaCode...)
	return append(dst, d.CoreNumber...)
}
