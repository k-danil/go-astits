package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// AAC represents an AAC audio descriptor: configuration of an MPEG-4 AAC coded
// audio elementary stream. The flag/type/additional-info fields are present
// only when HasFlags (descriptor_length > 1).
// Chapter: H.2 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type AAC struct {
	AdditionalInfo  []byte
	Header          Header
	ProfileAndLevel uint8
	AACType         uint8
	AACTypeFlag     bool
	SAOCDEFlag      bool
	HasFlags        bool
}

func newDescriptorAAC(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &AAC{
		Header: h,
	}
	dd = d

	if d.ProfileAndLevel, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	if i.Offset() >= offsetEnd {
		return
	}
	d.HasFlags = true

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.AACTypeFlag = b&0x80 > 0
	d.SAOCDEFlag = b&0x40 > 0

	if d.AACTypeFlag {
		if d.AACType, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
	}

	if d.AdditionalInfo, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *AAC) CalcLength() (n int) {
	n = 1
	if !d.HasFlags {
		return
	}
	n++
	if d.AACTypeFlag {
		n++
	}
	return n + len(d.AdditionalInfo)
}

func (d *AAC) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()), d.ProfileAndLevel)
	if !d.HasFlags {
		return dst
	}
	var b byte
	if d.AACTypeFlag {
		b |= 0x80
	}
	if d.SAOCDEFlag {
		b |= 0x40
	}
	dst = append(dst, b)
	if d.AACTypeFlag {
		dst = append(dst, d.AACType)
	}
	return append(dst, d.AdditionalInfo...)
}
