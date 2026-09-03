package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type ServiceAvailability struct {
	CellIDs          []uint16 `json:"cell_ids"`
	Header           Header   `json:"_header"`
	AvailabilityFlag bool     `json:"availability_flag"`
}

func newDescriptorServiceAvailability(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &ServiceAvailability{
		Header: h,
	}
	dd = d

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.AvailabilityFlag = b&0x80 > 0

	d.CellIDs = make([]uint16, (offsetEnd-i.Offset())/2)
	for idx := range d.CellIDs {
		var bs []byte
		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		d.CellIDs[idx] = binary.BigEndian.Uint16(bs)
	}
	return
}

func (d *ServiceAvailability) CalcLength() int {
	return 1 + 2*len(d.CellIDs)
}

func (d *ServiceAvailability) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	b := byte(0x7f)
	if d.AvailabilityFlag {
		b |= 0x80
	}
	dst = append(dst, b)
	for _, id := range d.CellIDs {
		dst = append(dst, byte(id>>8), byte(id))
	}
	return dst
}
