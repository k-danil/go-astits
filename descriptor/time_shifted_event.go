package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type TimeShiftedEvent struct {
	Header             Header `json:"_header"`
	ReferenceServiceID uint16 `json:"reference_service_id"`
	ReferenceEventID   uint16 `json:"reference_event_id"`
}

func newDescriptorTimeShiftedEvent(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &TimeShiftedEvent{
		Header: h,
	}
	dd = d

	var bs []byte
	if bs, err = i.NextBytesNoCopy(4); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d.ReferenceServiceID = binary.BigEndian.Uint16(bs[0:2])
	d.ReferenceEventID = binary.BigEndian.Uint16(bs[2:4])

	err = rejectTrailingBytes(i, offsetEnd)
	return
}

func (d *TimeShiftedEvent) CalcLength() int {
	return 4
}

func (d *TimeShiftedEvent) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	return append(dst,
		byte(d.ReferenceServiceID>>8), byte(d.ReferenceServiceID),
		byte(d.ReferenceEventID>>8), byte(d.ReferenceEventID))
}
