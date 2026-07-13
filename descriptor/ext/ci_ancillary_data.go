package ext

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// CIAncillaryData represents a CI ancillary data extension descriptor:
// ancillary data used to build Content Identifiers in companion-screen apps.
// Chapter: 6.4.1 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type CIAncillaryData struct {
	Data []byte `json:"ancillary_data_byte"`
}

func parseCIAncillaryData(i *bytesiter.Iterator, offsetEnd int) (d *CIAncillaryData, err error) {
	d = &CIAncillaryData{}
	if d.Data, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *CIAncillaryData) CalcLength() int {
	return len(d.Data)
}

func (d *CIAncillaryData) Append(dst []byte) []byte {
	return append(dst, d.Data...)
}
