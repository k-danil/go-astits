package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type PrivateDataIndicator struct {
	Header    Header `json:"_header"`
	Indicator uint32 `json:"private_data_indicator"`
}

func newDescriptorPrivateDataIndicator(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(4); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &PrivateDataIndicator{
		Header:    h,
		Indicator: binary.BigEndian.Uint32(bs),
	}
	dd = d

	err = rejectTrailingBytes(i, offsetEnd)
	return
}

func (d *PrivateDataIndicator) CalcLength() int {
	return 4
}

func (d *PrivateDataIndicator) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	return append(dst, byte(d.Indicator>>24), byte(d.Indicator>>16), byte(d.Indicator>>8), byte(d.Indicator))
}
