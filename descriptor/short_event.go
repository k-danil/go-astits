package descriptor

import (
	"fmt"

	"github.com/asticode/go-astikit"
)

// DescriptorShortEvent represents a short event descriptor
// Chapter: 6.2.37 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorShortEvent struct {
	EventName []byte
	Text      []byte
	Header    DescriptorHeader
	Language  [3]byte
}

func newDescriptorShortEvent(i *astikit.BytesIterator, h DescriptorHeader, _ int) (dd Descriptor, err error) {
	// Create descriptor
	d := &DescriptorShortEvent{
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

func (d *DescriptorShortEvent) length() uint8 {
	ret := 3 + 1 + 1 // language code and lengths
	ret += len(d.EventName)
	ret += len(d.Text)
	return uint8(ret)
}

func (d *DescriptorShortEvent) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.Language[:])

	b.Write(uint8(len(d.EventName)))
	b.Write(d.EventName)

	b.Write(uint8(len(d.Text)))
	b.Write(d.Text)

	return written, b.Err()
}
