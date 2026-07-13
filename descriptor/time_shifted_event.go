package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// TimeShiftedEvent represents a time shifted event descriptor: it points at the
// reference event a time-shifted event is a copy of, used in place of the short
// event descriptor.
// Chapter: 6.2.44 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type TimeShiftedEvent struct {
	Header             Header `json:"_header"`
	ReferenceServiceID uint16 `json:"reference_service_id"`
	ReferenceEventID   uint16 `json:"reference_event_id"`
}

func newDescriptorTimeShiftedEvent(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	d := &TimeShiftedEvent{
		Header: h,
	}
	dd = d

	var bs []byte
	if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d.ReferenceServiceID = binary.BigEndian.Uint16(bs[0:2])
	d.ReferenceEventID = binary.BigEndian.Uint16(bs[2:4])
	return
}

func (d *TimeShiftedEvent) CalcLength() int {
	return 4
}

func (d *TimeShiftedEvent) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst,
		byte(d.ReferenceServiceID>>8), byte(d.ReferenceServiceID),
		byte(d.ReferenceEventID>>8), byte(d.ReferenceEventID))
}
