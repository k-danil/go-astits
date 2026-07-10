package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// ExtensionMessage represents a message extension descriptor: a textual message
// (in a given language) a receiver may display to the user.
// Chapter: 6.4.7 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type ExtensionMessage struct {
	Text      []byte
	MessageID uint8
	Language  [3]byte
}

func newDescriptorExtensionMessage(i *bytesiter.Iterator, offsetEnd int) (d *ExtensionMessage, err error) {
	d = &ExtensionMessage{}

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

func (d *ExtensionMessage) CalcLength() int {
	return 4 + len(d.Text)
}

func (d *ExtensionMessage) Append(dst []byte) []byte {
	dst = append(dst, d.MessageID)
	dst = append(dst, d.Language[:]...)
	return append(dst, d.Text...)
}
