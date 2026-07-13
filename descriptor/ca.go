package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// CA represents a CA (conditional access) descriptor: it names a CA system and
// the PID carrying its management messages — EMM when the descriptor sits in a
// CAT, ECM when it sits in a PMT.
// Chapter: 2.6.16 | Link: ISO/IEC 13818-1
type CA struct {
	Private  []byte `json:"private_data_byte"`
	Header   Header `json:"_header"`
	SystemID uint16 `json:"CA_system_ID"`
	PID      uint16 `json:"CA_PID"`
}

func newDescriptorCA(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d := &CA{
		Header:   h,
		SystemID: binary.BigEndian.Uint16(bs[0:2]),
		PID:      binary.BigEndian.Uint16(bs[2:4]) & 0x1fff,
	}
	dd = d
	if offsetEnd > i.Offset() {
		if d.Private, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *CA) CalcLength() int {
	return 4 + len(d.Private)
}

func (d *CA) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, byte(d.SystemID>>8), byte(d.SystemID))
	dst = append(dst, 0xe0|byte(d.PID>>8)&0x1f, byte(d.PID))
	return append(dst, d.Private...)
}
