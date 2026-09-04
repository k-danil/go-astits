package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type CAIdentifier struct {
	Header    Header   `json:"_header"`
	SystemIDs []uint16 `json:"CA_system_id"`
}

func newDescriptorCAIdentifier(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &CAIdentifier{
		Header:    h,
		SystemIDs: make([]uint16, (offsetEnd-i.Offset())/2),
	}
	dd = d

	for idx := range d.SystemIDs {
		var bs []byte
		if bs, err = i.NextBytesNoCopy(2); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		d.SystemIDs[idx] = binary.BigEndian.Uint16(bs)
	}

	err = rejectTrailingBytes(i, offsetEnd)
	return
}

func (d *CAIdentifier) CalcLength() int {
	return 2 * len(d.SystemIDs)
}

func (d *CAIdentifier) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	for _, id := range d.SystemIDs {
		dst = append(dst, byte(id>>8), byte(id))
	}
	return dst
}
