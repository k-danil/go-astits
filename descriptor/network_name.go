package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/dvbtext"
	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type NetworkName struct {
	Header Header       `json:"_header"`
	Name   dvbtext.Text `json:"network_name"`
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
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	return append(dst, d.Name...)
}
