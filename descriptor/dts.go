package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// DTS represents a DTS audio descriptor: configuration of a DTS coded audio
// elementary stream (sample/bit rate, frame sizing, surround mode).
// Chapter: G.2 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DTS struct {
	AdditionalInfo       []byte
	Header               Header
	SampleRateCode       uint8
	BitRateCode          uint8
	NBLKS                uint8
	FSize                uint16
	SurroundMode         uint8
	ExtendedSurroundFlag uint8
	LFEFlag              bool
}

func newDescriptorDTS(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &DTS{
		Header: h,
	}
	dd = d

	var bs []byte
	if bs, err = i.NextBytesNoCopy(5); err != nil || len(bs) < 5 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	v := uint64(bs[0])<<32 | uint64(bs[1])<<24 | uint64(bs[2])<<16 | uint64(bs[3])<<8 | uint64(bs[4])
	d.SampleRateCode = uint8(v >> 36 & 0x0f)
	d.BitRateCode = uint8(v >> 30 & 0x3f)
	d.NBLKS = uint8(v >> 23 & 0x7f)
	d.FSize = uint16(v >> 9 & 0x3fff)
	d.SurroundMode = uint8(v >> 3 & 0x3f)
	d.LFEFlag = v&0x04 > 0
	d.ExtendedSurroundFlag = uint8(v & 0x03)

	if d.AdditionalInfo, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *DTS) CalcLength() int {
	return 5 + len(d.AdditionalInfo)
}

func (d *DTS) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	v := uint64(d.SampleRateCode&0x0f)<<36 | uint64(d.BitRateCode&0x3f)<<30 |
		uint64(d.NBLKS&0x7f)<<23 | uint64(d.FSize&0x3fff)<<9 |
		uint64(d.SurroundMode&0x3f)<<3 | uint64(d.ExtendedSurroundFlag&0x03)
	if d.LFEFlag {
		v |= 0x04
	}
	dst = append(dst, byte(v>>32), byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
	return append(dst, d.AdditionalInfo...)
}
