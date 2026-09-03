package ext

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type DTSNeural struct {
	AdditionalInfo []byte `json:"additional_info"`
	ConfigID       uint8  `json:"config_id"`
}

func parseDTSNeural(i *bytesiter.Iterator, offsetEnd int) (d *DTSNeural, err error) {
	d = &DTSNeural{}

	if d.ConfigID, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	if i.Offset() < offsetEnd {
		if d.AdditionalInfo, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *DTSNeural) CalcLength() int {
	return 1 + len(d.AdditionalInfo)
}

func (d *DTSNeural) Append(dst []byte) []byte {
	dst = append(dst, d.ConfigID)
	return append(dst, d.AdditionalInfo...)
}
