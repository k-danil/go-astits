package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type SL struct {
	ESID   uint16 `json:"ES_ID"`
	Header Header `json:"_header"`
}

func newDescriptorSL(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &SL{
		Header: h,
		ESID:   binary.BigEndian.Uint16(bs),
	}
	dd = d

	err = rejectTrailingBytes(i, offsetEnd)
	return
}

func (d *SL) CalcLength() int {
	return 2
}

func (d *SL) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	return append(dst, byte(d.ESID>>8), byte(d.ESID))
}
