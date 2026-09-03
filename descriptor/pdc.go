package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

// ProgrammeIdentificationLabel packs day/month/hour/minute (EN 300 231), not a plain number.
type PDC struct {
	Header                       Header `json:"_header"`
	ProgrammeIdentificationLabel uint32 `json:"programme_identification_label"`
}

func newDescriptorPDC(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	d := &PDC{
		Header: h,
	}
	dd = d

	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d.ProgrammeIdentificationLabel = uint32(bs[0]&0x0f)<<16 | uint32(bs[1])<<8 | uint32(bs[2])
	return
}

func (d *PDC) CalcLength() int {
	return 3
}

func (d *PDC) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst,
		0xf0|byte(d.ProgrammeIdentificationLabel>>16)&0x0f,
		byte(d.ProgrammeIdentificationLabel>>8),
		byte(d.ProgrammeIdentificationLabel))
}
