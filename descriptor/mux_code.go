package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// MuxCode is the MPEG-2 systems Muxcode descriptor (ISO/IEC 13818-1).
type MuxCode struct {
	MuxCodeTableEntries []byte
	Header              Header
}

func newDescriptorMuxCode(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &MuxCode{
		Header: h,
	}
	dd = d

	if i.Offset() < offsetEnd {
		if d.MuxCodeTableEntries, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *MuxCode) CalcLength() int {
	return len(d.MuxCodeTableEntries)
}

func (d *MuxCode) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, d.MuxCodeTableEntries...)
}
