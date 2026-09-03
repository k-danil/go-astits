package psi

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type DIT struct {
	TransitionFlag bool `json:"transition_flag"`
}

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

func (d *DIT) appendSection(dst []byte) []byte {
	b := byte(0x7f)
	if d.TransitionFlag {
		b |= 0x80
	}
	return append(dst, b)
}
