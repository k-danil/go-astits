package ext

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// CPIdentifier represents a CP identifier extension descriptor: the CP
// systems a bouquet, service or event is associated with.
// Chapter: 6.4.3 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type CPIdentifier struct {
	SystemIDs []uint16 `json:"_CP_system_ids"`
}

func parseCPIdentifier(i *bytesiter.Iterator, offsetEnd int) (d *CPIdentifier, err error) {
	d = &CPIdentifier{
		SystemIDs: make([]uint16, (offsetEnd-i.Offset())/2),
	}
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

func (d *CPIdentifier) CalcLength() int {
	return 2 * len(d.SystemIDs)
}

func (d *CPIdentifier) Append(dst []byte) []byte {
	for _, id := range d.SystemIDs {
		dst = append(dst, byte(id>>8), byte(id))
	}
	return dst
}
