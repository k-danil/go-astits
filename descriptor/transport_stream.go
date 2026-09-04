package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type TransportStream struct {
	Header Header `json:"_header"`
	Data   []byte `json:"byte"`
}

func newDescriptorTransportStream(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &TransportStream{
		Header: h,
	}
	dd = d

	if d.Data, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *TransportStream) CalcLength() int {
	return len(d.Data)
}

func (d *TransportStream) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	return append(dst, d.Data...)
}
