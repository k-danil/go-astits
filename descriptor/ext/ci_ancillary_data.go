package ext

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

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
