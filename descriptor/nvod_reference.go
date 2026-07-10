package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// NVODReference represents an NVOD reference descriptor: the services that
// together form a near-video-on-demand service.
// Chapter: 6.2.26 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type NVODReference struct {
	Items  []NVODReferenceItem
	Header Header
}

// NVODReferenceItem represents one entry of an NVOD reference descriptor
type NVODReferenceItem struct {
	TransportStreamID uint16
	OriginalNetworkID uint16
	ServiceID         uint16
}

func newDescriptorNVODReference(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &NVODReference{
		Header: h,
		Items:  make([]NVODReferenceItem, (offsetEnd-i.Offset())/6),
	}
	dd = d

	for idx := range d.Items {
		var bs []byte
		if bs, err = i.NextBytesNoCopy(6); err != nil || len(bs) < 6 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		d.Items[idx].TransportStreamID = binary.BigEndian.Uint16(bs[0:2])
		d.Items[idx].OriginalNetworkID = binary.BigEndian.Uint16(bs[2:4])
		d.Items[idx].ServiceID = binary.BigEndian.Uint16(bs[4:6])
	}
	return
}

func (d *NVODReference) CalcLength() int {
	return 6 * len(d.Items)
}

func (d *NVODReference) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	for _, item := range d.Items {
		dst = append(dst,
			byte(item.TransportStreamID>>8), byte(item.TransportStreamID),
			byte(item.OriginalNetworkID>>8), byte(item.OriginalNetworkID),
			byte(item.ServiceID>>8), byte(item.ServiceID))
	}
	return dst
}
