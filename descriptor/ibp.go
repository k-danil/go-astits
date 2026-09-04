package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
	"github.com/k-danil/go-astits/v3/internal/util"
)

type IBP struct {
	Header           Header `json:"_header"`
	MaxGOPLength     uint16 `json:"max_gop_length"`
	ClosedGOPFlag    bool   `json:"closed_gop_flag"`
	IdenticalGOPFlag bool   `json:"identical_gop_flag"`
}

func newDescriptorIBP(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &IBP{
		Header:           h,
		ClosedGOPFlag:    bs[0]&0x80 > 0,
		IdenticalGOPFlag: bs[0]&0x40 > 0,
		MaxGOPLength:     uint16(bs[0]&0x3f)<<8 | uint16(bs[1]),
	}
	dd = d

	err = rejectTrailingBytes(i, offsetEnd)
	return
}

func (d *IBP) CalcLength() int {
	return 2
}

func (d *IBP) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	dst = append(dst, util.B2U(d.ClosedGOPFlag)<<7|util.B2U(d.IdenticalGOPFlag)<<6|byte(d.MaxGOPLength>>8)&0x3f)
	return append(dst, byte(d.MaxGOPLength))
}
