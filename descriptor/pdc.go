package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// PDC represents a PDC descriptor: the 20-bit Programme Identification Label
// (day/month/hour/minute of the first published start time), per EN 300 231.
// Chapter: 6.2.30 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type PDC struct {
	Header                       Header
	ProgrammeIdentificationLabel uint32
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
