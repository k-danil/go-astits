package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// TransportStream represents a transport stream descriptor: carried in the TSDT
// to flag compliance with an MPEG-based system; the bytes are typically the
// ASCII "DVB".
// Chapter: 6.2.46 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type TransportStream struct {
	Header Header
	Data   []byte
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
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, d.Data...)
}
