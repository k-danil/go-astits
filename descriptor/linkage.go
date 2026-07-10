package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// Linkage represents a linkage descriptor: the service that provides extra
// information about the entity the descriptor sits in. The type-specific
// linkage info (mobile hand-over / event / extended-event, keyed by
// LinkageType) plus the trailing private data are kept raw in Data.
// Chapter: 6.2.19 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type Linkage struct {
	Data              []byte
	Header            Header
	TransportStreamID uint16
	OriginalNetworkID uint16
	ServiceID         uint16
	LinkageType       uint8
}

func newDescriptorLinkage(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &Linkage{
		Header: h,
	}
	dd = d

	var bs []byte
	if bs, err = i.NextBytesNoCopy(7); err != nil || len(bs) < 7 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d.TransportStreamID = binary.BigEndian.Uint16(bs[0:2])
	d.OriginalNetworkID = binary.BigEndian.Uint16(bs[2:4])
	d.ServiceID = binary.BigEndian.Uint16(bs[4:6])
	d.LinkageType = bs[6]

	if d.Data, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *Linkage) CalcLength() int {
	return 7 + len(d.Data)
}

func (d *Linkage) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst,
		byte(d.TransportStreamID>>8), byte(d.TransportStreamID),
		byte(d.OriginalNetworkID>>8), byte(d.OriginalNetworkID),
		byte(d.ServiceID>>8), byte(d.ServiceID),
		d.LinkageType)
	return append(dst, d.Data...)
}
