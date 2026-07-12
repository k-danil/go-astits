package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// MPEGExtension is the MPEG-2 systems extension_descriptor (ISO/IEC 13818-1).
type MPEGExtension struct {
	Body      []byte
	Header    Header
	Extension uint8
}

func newDescriptorMPEGExtension(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	d := &MPEGExtension{
		Header:    h,
		Extension: b,
	}
	dd = d

	if i.Offset() < offsetEnd {
		if d.Body, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *MPEGExtension) CalcLength() int {
	return 1 + len(d.Body)
}

func (d *MPEGExtension) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, d.Extension)
	return append(dst, d.Body...)
}
