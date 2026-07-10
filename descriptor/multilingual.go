package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// readLangText reads a 24-bit ISO-639 language code into lang, then a length-
// prefixed text run (an owned copy) into *text — the item shape shared by the
// multilingual descriptors.
func readLangText(i *bytesiter.Iterator, lang []byte, text *[]byte) (err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	copy(lang, bs)

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	if *text, err = i.NextBytes(int(b)); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}
