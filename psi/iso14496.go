package psi

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// ISO14496Section represents an ISO_IEC_14496_section (ISO/IEC 13818-1 §2.11):
// SL-packetized or FlexMux data carried in sections. The table_id (0x04/0x05/0x08)
// distinguishes the stream type; the body is defined in ISO/IEC 14496-1 and is
// carried verbatim.
type ISO14496Section struct {
	Data []byte
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
