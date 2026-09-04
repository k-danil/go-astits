package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type StreamIdentifier struct {
	Header       Header `json:"_header"`
	ComponentTag uint8  `json:"component_tag"`
}

func newDescriptorStreamIdentifier(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
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

	err = rejectTrailingBytes(i, offsetEnd)
	return
}

func (d *StreamIdentifier) CalcLength() int {
	return 1
}

func (d *StreamIdentifier) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	return append(dst, d.ComponentTag)
}
