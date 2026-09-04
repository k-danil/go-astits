package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/dvbtext"
	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type ShortEvent struct {
	EventName dvbtext.Text `json:"event_name"`
	Text      dvbtext.Text `json:"text_char"`
	Header    Header       `json:"_header"`
	Language  dvbtext.Code `json:"ISO_639_language_code"`
}

func newDescriptorShortEvent(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &ShortEvent{
		Header: h,
	}
	dd = d

	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	copy(d.Language[:], bs)

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	eventLength := int(b)

	if d.EventName, err = i.NextBytes(eventLength); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	textLength := int(b)

	if d.Text, err = i.NextBytes(textLength); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	err = rejectTrailingBytes(i, offsetEnd)
	return
}

func (d *ShortEvent) CalcLength() int {
	return 3 + 1 + 1 + len(d.EventName) + len(d.Text)
}

func (d *ShortEvent) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	dst = append(dst, d.Language[:]...)
	dst = append(dst, uint8(len(d.EventName)))
	dst = append(dst, d.EventName...)
	dst = append(dst, uint8(len(d.Text)))
	return append(dst, d.Text...)
}
