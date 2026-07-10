package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// CountryAvailability represents a country availability descriptor: whether a
// service is intended to be available (or not) in the listed countries.
// Chapter: 6.2.10 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type CountryAvailability struct {
	Countries        [][3]byte
	Header           Header
	AvailabilityFlag bool
}

func newDescriptorCountryAvailability(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &CountryAvailability{
		Header: h,
	}
	dd = d

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.AvailabilityFlag = b&0x80 > 0

	d.Countries = make([][3]byte, (offsetEnd-i.Offset())/3)
	for idx := range d.Countries {
		var bs []byte
		if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		copy(d.Countries[idx][:], bs)
	}
	return
}

func (d *CountryAvailability) CalcLength() int {
	return 1 + 3*len(d.Countries)
}

func (d *CountryAvailability) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	b := byte(0x7f)
	if d.AvailabilityFlag {
		b |= 0x80
	}
	dst = append(dst, b)
	for _, c := range d.Countries {
		dst = append(dst, c[:]...)
	}
	return dst
}
