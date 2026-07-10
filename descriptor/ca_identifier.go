package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// CAIdentifier represents a CA identifier descriptor: it lists the CA system
// ids a bouquet, service or event is associated with.
// Chapter: 6.2.5 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type CAIdentifier struct {
	Header    Header
	SystemIDs []uint16
}

func newDescriptorCAIdentifier(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &CAIdentifier{
		Header:    h,
		SystemIDs: make([]uint16, (offsetEnd-i.Offset())/2),
	}
	dd = d

	for idx := range d.SystemIDs {
		var bs []byte
		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		d.SystemIDs[idx] = binary.BigEndian.Uint16(bs)
	}
	return
}

func (d *CAIdentifier) CalcLength() int {
	return 2 * len(d.SystemIDs)
}

func (d *CAIdentifier) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	for _, id := range d.SystemIDs {
		dst = append(dst, byte(id>>8), byte(id))
	}
	return dst
}
