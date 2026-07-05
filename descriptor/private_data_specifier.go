package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// PrivateDataSpecifier represents a private data specifier descriptor
type PrivateDataSpecifier struct {
	Header    Header
	Specifier uint32
}

func newDescriptorPrivateDataSpecifier(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &PrivateDataSpecifier{
		Header:    h,
		Specifier: binary.BigEndian.Uint32(bs),
	}
	dd = d
	return
}

func (*PrivateDataSpecifier) CalcLength() int {
	return 4
}

func (d *PrivateDataSpecifier) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, byte(d.Specifier>>24), byte(d.Specifier>>16), byte(d.Specifier>>8), byte(d.Specifier))
}
