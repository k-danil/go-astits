package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type ServiceList struct {
	Items  []ServiceListItem `json:"_items"`
	Header Header            `json:"_header"`
}

type ServiceListItem struct {
	ServiceID   uint16 `json:"service_id"`
	ServiceType uint8  `json:"service_type"`
}

func newDescriptorServiceList(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &ServiceList{
		Header: h,
		Items:  make([]ServiceListItem, (offsetEnd-i.Offset())/3),
	}
	dd = d

	for idx := range d.Items {
		var bs []byte
		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		d.Items[idx].ServiceID = binary.BigEndian.Uint16(bs)

		var b byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.Items[idx].ServiceType = b
	}
	return
}

func (d *ServiceList) CalcLength() int {
	return 3 * len(d.Items)
}

func (d *ServiceList) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	for _, item := range d.Items {
		dst = append(dst, byte(item.ServiceID>>8), byte(item.ServiceID), item.ServiceType)
	}
	return dst
}
