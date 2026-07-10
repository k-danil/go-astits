package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// FrequencyList represents a frequency list descriptor: the additional
// frequencies a multiplex is transmitted on. CodingType selects the delivery
// system the raw centre-frequency values are coded for.
// Chapter: 6.2.17 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type FrequencyList struct {
	Frequencies []uint32
	Header      Header
	CodingType  uint8
}

func newDescriptorFrequencyList(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &FrequencyList{
		Header: h,
	}
	dd = d

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.CodingType = b & 0x03

	d.Frequencies = make([]uint32, (offsetEnd-i.Offset())/4)
	for idx := range d.Frequencies {
		var bs []byte
		if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		d.Frequencies[idx] = binary.BigEndian.Uint32(bs)
	}
	return
}

func (d *FrequencyList) CalcLength() int {
	return 1 + 4*len(d.Frequencies)
}

func (d *FrequencyList) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, 0xfc|d.CodingType&0x03)
	for _, f := range d.Frequencies {
		dst = append(dst, byte(f>>24), byte(f>>16), byte(f>>8), byte(f))
	}
	return dst
}
