package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/dvbtext"
	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type BouquetName struct {
	Header Header       `json:"_header"`
	Name   dvbtext.Text `json:"char"`
}

func newDescriptorBouquetName(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &BouquetName{
		Header: h,
	}
	dd = d

	if d.Name, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *BouquetName) CalcLength() int {
	return len(d.Name)
}

func (d *BouquetName) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, d.Name...)
}
