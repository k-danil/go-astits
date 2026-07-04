package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// StreamIdentifier represents a stream identifier descriptor
// Chapter: 6.2.39 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type StreamIdentifier struct {
	Header       Header
	ComponentTag uint8
}

func newDescriptorStreamIdentifier(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d := &StreamIdentifier{
		Header:       h,
		ComponentTag: b,
	}
	dd = d
	return
}

func (*StreamIdentifier) CalcLength() int {
	return 1
}

func (d *StreamIdentifier) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, d.ComponentTag)
}
