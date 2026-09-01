package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/dvbtext"
	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

func readLangText(i *bytesiter.Iterator, lang *dvbtext.Code, text *dvbtext.Text) (err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	copy(lang[:], bs)

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
