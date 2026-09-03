package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type ServiceMove struct {
	Header               Header `json:"_header"`
	NewOriginalNetworkID uint16 `json:"new_original_network_id"`
	NewTransportStreamID uint16 `json:"new_transport_stream_id"`
	NewServiceID         uint16 `json:"new_service_id"`
}

func newDescriptorServiceMove(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	d := &ServiceMove{
		Header: h,
	}
	dd = d

	var bs []byte
	if bs, err = i.NextBytesNoCopy(6); err != nil || len(bs) < 6 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d.NewOriginalNetworkID = binary.BigEndian.Uint16(bs[0:2])
	d.NewTransportStreamID = binary.BigEndian.Uint16(bs[2:4])
	d.NewServiceID = binary.BigEndian.Uint16(bs[4:6])
	return
}

func (d *ServiceMove) CalcLength() int {
	return 6
}

func (d *ServiceMove) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst,
		byte(d.NewOriginalNetworkID>>8), byte(d.NewOriginalNetworkID),
		byte(d.NewTransportStreamID>>8), byte(d.NewTransportStreamID),
		byte(d.NewServiceID>>8), byte(d.NewServiceID))
}
