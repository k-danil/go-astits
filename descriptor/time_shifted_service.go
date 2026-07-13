package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// TimeShiftedService represents a time shifted service descriptor: it points at
// the reference service a time-shifted service is a copy of, used in place of
// the service descriptor.
// Chapter: 6.2.45 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type TimeShiftedService struct {
	Header             Header `json:"_header"`
	ReferenceServiceID uint16 `json:"reference_service_id"`
}

func newDescriptorTimeShiftedService(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	d := &TimeShiftedService{
		Header: h,
	}
	dd = d

	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d.ReferenceServiceID = binary.BigEndian.Uint16(bs)
	return
}

func (d *TimeShiftedService) CalcLength() int {
	return 2
}

func (d *TimeShiftedService) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, byte(d.ReferenceServiceID>>8), byte(d.ReferenceServiceID))
}
