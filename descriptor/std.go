package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

// STD is the MPEG-2 systems STD_descriptor (ISO/IEC 13818-1).
type STD struct {
	Header        Header
	LeakValidFlag bool
}

func newDescriptorSTD(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d := &STD{
		Header:        h,
		LeakValidFlag: b&0x01 > 0,
	}
	dd = d
	return
}

func (*STD) CalcLength() int {
	return 1
}

func (d *STD) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, 0xfe|util.B2U(d.LeakValidFlag))
}
