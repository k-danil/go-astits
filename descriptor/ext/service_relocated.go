package ext

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type ServiceRelocated struct {
	OldOriginalNetworkID uint16 `json:"old_original_network_id"`
	OldTransportStreamID uint16 `json:"old_transport_stream_id"`
	OldServiceID         uint16 `json:"old_service_id"`
}

func parseServiceRelocated(i *bytesiter.Iterator, _ int) (d *ServiceRelocated, err error) {
	d = &ServiceRelocated{}

	var bs []byte
	if bs, err = i.NextBytesNoCopy(6); err != nil || len(bs) < 6 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d.OldOriginalNetworkID = binary.BigEndian.Uint16(bs[0:2])
	d.OldTransportStreamID = binary.BigEndian.Uint16(bs[2:4])
	d.OldServiceID = binary.BigEndian.Uint16(bs[4:6])
	return
}

func (d *ServiceRelocated) CalcLength() int {
	return 6
}

func (d *ServiceRelocated) Append(dst []byte) []byte {
	return append(dst,
		byte(d.OldOriginalNetworkID>>8), byte(d.OldOriginalNetworkID),
		byte(d.OldTransportStreamID>>8), byte(d.OldTransportStreamID),
		byte(d.OldServiceID>>8), byte(d.OldServiceID))
}
