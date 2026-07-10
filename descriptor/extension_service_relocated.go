package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// ExtensionServiceRelocated represents a service relocated extension
// descriptor: the previous identifiers of a service that has moved, so an IRD
// can track it to its new location.
// Chapter: 6.4.9 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type ExtensionServiceRelocated struct {
	OldOriginalNetworkID uint16
	OldTransportStreamID uint16
	OldServiceID         uint16
}

func newDescriptorExtensionServiceRelocated(i *bytesiter.Iterator, _ int) (d *ExtensionServiceRelocated, err error) {
	d = &ExtensionServiceRelocated{}

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

func (d *ExtensionServiceRelocated) CalcLength() int {
	return 6
}

func (d *ExtensionServiceRelocated) Append(dst []byte) []byte {
	return append(dst,
		byte(d.OldOriginalNetworkID>>8), byte(d.OldOriginalNetworkID),
		byte(d.OldTransportStreamID>>8), byte(d.OldTransportStreamID),
		byte(d.OldServiceID>>8), byte(d.OldServiceID))
}
