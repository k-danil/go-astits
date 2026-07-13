package ext

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// CP represents a content protection (CP) extension descriptor: the CP
// system and the PID carrying its program-related information.
// Chapter: 6.4.2 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type CP struct {
	PrivateData []byte `json:"private_data"`
	CPSystemID  uint16 `json:"CP_system_id"`
	CPPID       uint16 `json:"CP_PID"`
}

func parseCP(i *bytesiter.Iterator, offsetEnd int) (d *CP, err error) {
	d = &CP{}

	var bs []byte
	if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d.CPSystemID = binary.BigEndian.Uint16(bs[0:2])
	d.CPPID = binary.BigEndian.Uint16(bs[2:4]) & 0x1fff

	if i.Offset() < offsetEnd {
		if d.PrivateData, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *CP) CalcLength() int {
	return 4 + len(d.PrivateData)
}

func (d *CP) Append(dst []byte) []byte {
	dst = append(dst, byte(d.CPSystemID>>8), byte(d.CPSystemID),
		0xe0|byte(d.CPPID>>8)&0x1f, byte(d.CPPID))
	return append(dst, d.PrivateData...)
}
