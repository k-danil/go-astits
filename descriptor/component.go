package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/internal/bytesiter"
)

// Component represents a component descriptor
// Chapter: 6.2.8 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type Component struct {
	Header             Header
	ISO639LanguageCode [3]byte
	Text               []byte
	ComponentTag       uint8
	ComponentType      uint8
	StreamContent      uint8
	StreamContentExt   uint8
}

func newDescriptorComponent(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	// Init
	d := &Component{
		Header: h,
	}
	dd = d

	// Get next byte
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Stream content ext
	d.StreamContentExt = b >> 4

	// Stream content
	d.StreamContent = b & 0xf

	// Get next byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Component type
	d.ComponentType = b

	// Get next byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Component tag
	d.ComponentTag = b

	// ISO639 language code
	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	copy(d.ISO639LanguageCode[:], bs)

	// Text
	if i.Offset() < offsetEnd {
		if d.Text, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *Component) CalcLength() int {
	return 6 + len(d.Text)
}

func (d *Component) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, d.StreamContentExt<<4|d.StreamContent&0xf)
	dst = append(dst, d.ComponentType, d.ComponentTag)
	dst = append(dst, d.ISO639LanguageCode[:]...)
	return append(dst, d.Text...)
}
