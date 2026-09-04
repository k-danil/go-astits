package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
	"github.com/k-danil/go-astits/v3/internal/util"
)

type STD struct {
	Header        Header `json:"_header"`
	LeakValidFlag bool   `json:"leak_valid_flag"`
}

func newDescriptorSTD(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
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

	err = rejectTrailingBytes(i, offsetEnd)
	return
}

func (d *STD) CalcLength() int {
	return 1
}

func (d *STD) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	return append(dst, 0xfe|util.B2U(d.LeakValidFlag))
}
