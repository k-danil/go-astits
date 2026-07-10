package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// ServiceList represents a service list descriptor: the services in a transport
// stream by id and type.
// Chapter: 6.2.35 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type ServiceList struct {
	Items  []ServiceListItem
	Header Header
}

// ServiceListItem represents one entry of a service list descriptor
type ServiceListItem struct {
	ServiceID   uint16
	ServiceType uint8
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
