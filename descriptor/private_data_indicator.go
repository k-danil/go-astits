package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// PrivateDataIndicator represents a private data Indicator descriptor
type PrivateDataIndicator struct {
	Header    Header
	Indicator uint32
}

func newDescriptorPrivateDataIndicator(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &PrivateDataIndicator{
		Header:    h,
		Indicator: binary.BigEndian.Uint32(bs),
	}
	dd = d
	return
}

func (*PrivateDataIndicator) CalcLength() int {
	return 4
}

func (d *PrivateDataIndicator) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, byte(d.Indicator>>24), byte(d.Indicator>>16), byte(d.Indicator>>8), byte(d.Indicator))
}
