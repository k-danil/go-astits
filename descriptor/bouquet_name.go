package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// BouquetName represents a bouquet name descriptor
// Chapter: 6.2.4 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type BouquetName struct {
	Header Header
	Name   []byte
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
