package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// NetworkName represents a network name descriptor
// Chapter: 6.2.27 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type NetworkName struct {
	Header Header
	Name   []byte
}

func newDescriptorNetworkName(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &NetworkName{
		Header: h,
	}
	dd = d

	if d.Name, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *NetworkName) CalcLength() int {
	return len(d.Name)
}

func (d *NetworkName) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, d.Name...)
}
