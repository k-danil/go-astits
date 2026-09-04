package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type IOD struct {
	InitialObjectDescriptor []byte `json:"InitialObjectDescriptor"`
	Header                  Header `json:"_header"`
	ScopeOfIODLabel         uint8  `json:"Scope_of_IOD_label"`
	IODLabel                uint8  `json:"IOD_label"`
}

func newDescriptorIOD(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d := &IOD{
		Header:          h,
		ScopeOfIODLabel: b,
	}
	dd = d

	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.IODLabel = b

	if i.Offset() < offsetEnd {
		if d.InitialObjectDescriptor, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *IOD) CalcLength() int {
	return 2 + len(d.InitialObjectDescriptor)
}

func (d *IOD) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	dst = append(dst, d.ScopeOfIODLabel, d.IODLabel)
	return append(dst, d.InitialObjectDescriptor...)
}
