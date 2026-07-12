package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// SL is the MPEG-2 systems SL_descriptor (ISO/IEC 13818-1).
type SL struct {
	ESID   uint16
	Header Header
}

func newDescriptorSL(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &SL{
		Header: h,
		ESID:   binary.BigEndian.Uint16(bs),
	}
	dd = d
	return
}

func (*SL) CalcLength() int {
	return 2
}

func (d *SL) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, byte(d.ESID>>8), byte(d.ESID))
}
