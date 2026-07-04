package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/internal/bytesiter"
)

// ShortEvent represents a short event descriptor
// Chapter: 6.2.37 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type ShortEvent struct {
	EventName []byte
	Text      []byte
	Header    Header
	Language  [3]byte
}

func newDescriptorShortEvent(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	// Create descriptor
	d := &ShortEvent{
		Header: h,
	}
	dd = d

	// Language
	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	copy(d.Language[:], bs)

	// Get next byte
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Event length
	eventLength := int(b)

	// Event name
	if d.EventName, err = i.NextBytes(eventLength); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	// Get next byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Text length
	textLength := int(b)

	// Text
	if d.Text, err = i.NextBytes(textLength); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *ShortEvent) CalcLength() int {
	return 3 + 1 + 1 + len(d.EventName) + len(d.Text) // language code and lengths
}

func (d *ShortEvent) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, d.Language[:]...)
	dst = append(dst, uint8(len(d.EventName)))
	dst = append(dst, d.EventName...)
	dst = append(dst, uint8(len(d.Text)))
	return append(dst, d.Text...)
}
