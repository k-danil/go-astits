package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// FmxBufferSize is the MPEG-2 systems FmxBufferSize_descriptor (ISO/IEC 13818-1).
type FmxBufferSize struct {
	Body   []byte `json:"FlexMuxBufferDescriptor"` // FlexMux buffer sub-descriptors, defined in ISO/IEC 14496-1
	Header Header `json:"_header"`
}

func newDescriptorFmxBufferSize(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &FmxBufferSize{Header: h}
	dd = d

	if i.Offset() < offsetEnd {
		if d.Body, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *FmxBufferSize) CalcLength() int {
	return len(d.Body)
}

func (d *FmxBufferSize) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, d.Body...)
}
