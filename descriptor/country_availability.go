package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/dvbtext"
	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type CountryAvailability struct {
	Countries        []dvbtext.Code `json:"country_code"`
	Header           Header         `json:"_header"`
	AvailabilityFlag bool           `json:"country_availability_flag"`
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

	d.Countries = make([]dvbtext.Code, (offsetEnd-i.Offset())/3)
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
