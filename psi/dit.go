package psi

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// DIT represents a DIT: the discontinuity information table marks a
// discontinuity in a partial (recorded) transport stream via a single
// transition flag. It has neither a syntax header nor a CRC.
// Page: 40 | Chapter: 5.2.9 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DIT struct {
	TransitionFlag bool `json:"transition_flag"`
}

// parseDITSection parses a DIT section
func parseDITSection(i *bytesiter.Iterator) (d *DIT, err error) {
	d = &DIT{}

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.TransitionFlag = b&0x80 > 0
	return
}

func (d *DIT) CalcSectionLength() int { return 1 }

// appendSection appends the DIT body: transition_flag then reserved_future_use
// (7 bits, set to 1). §5.2.9, section_length is fixed at 0x001.
func (d *DIT) appendSection(dst []byte) []byte {
	b := byte(0x7f)
	if d.TransitionFlag {
		b |= 0x80
	}
	return append(dst, b)
}
