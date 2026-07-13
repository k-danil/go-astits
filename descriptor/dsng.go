package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// DSNG represents a DSNG descriptor: carried in the TSDT of Digital Satellite
// News Gathering transmissions; its bytes are defined by EN 301 210 and start
// with the ASCII "CONA".
// Chapter: 6.2.14 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DSNG struct {
	Header Header `json:"_header"`
	Data   []byte `json:"byte"`
}

func newDescriptorDSNG(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &DSNG{
		Header: h,
	}
	dd = d

	if d.Data, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *DSNG) CalcLength() int {
	return len(d.Data)
}

func (d *DSNG) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, d.Data...)
}
