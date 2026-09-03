package psi

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type ISO14496Section struct {
	Data []byte `json:"_data"`
}

func parseISO14496Section(i *bytesiter.Iterator, offsetSectionsEnd int) (d *ISO14496Section, err error) {
	d = &ISO14496Section{}
	length := offsetSectionsEnd - i.Offset()
	if length <= 0 {
		return
	}
	if d.Data, err = i.NextBytes(length); err != nil {
		err = fmt.Errorf("astits: fetching ISO_IEC_14496 section bytes failed: %w", err)
		return
	}
	return
}

func (d *ISO14496Section) CalcSectionLength() int { return len(d.Data) }

func (d *ISO14496Section) appendSection(dst []byte) []byte {
	return append(dst, d.Data...)
}
