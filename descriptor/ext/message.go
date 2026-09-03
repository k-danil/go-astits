package ext

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/dvbtext"
	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type Message struct {
	Text      dvbtext.Text `json:"text_char"`
	MessageID uint8        `json:"message_id"`
	Language  dvbtext.Code `json:"ISO_639_language_code"`
}

func parseMessage(i *bytesiter.Iterator, offsetEnd int) (d *Message, err error) {
	d = &Message{}

	if d.MessageID, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	copy(d.Language[:], bs)

	if d.Text, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *Message) CalcLength() int {
	return 4 + len(d.Text)
}

func (d *Message) Append(dst []byte) []byte {
	dst = append(dst, d.MessageID)
	dst = append(dst, d.Language[:]...)
	return append(dst, d.Text...)
}
