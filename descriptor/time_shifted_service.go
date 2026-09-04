package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type TimeShiftedService struct {
	Header             Header `json:"_header"`
	ReferenceServiceID uint16 `json:"reference_service_id"`
}

func newDescriptorTimeShiftedService(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &TimeShiftedService{
		Header: h,
	}
	dd = d

	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d.ReferenceServiceID = binary.BigEndian.Uint16(bs)

	err = rejectTrailingBytes(i, offsetEnd)
	return
}

func (d *TimeShiftedService) CalcLength() int {
	return 2
}

func (d *TimeShiftedService) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	return append(dst, byte(d.ReferenceServiceID>>8), byte(d.ReferenceServiceID))
}
