package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// DataBroadcastID represents a data broadcast id descriptor: a short form of
// the data broadcast descriptor for the PMT component loop.
// Chapter: 6.2.12 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DataBroadcastID struct {
	Selector        []byte
	Header          Header
	DataBroadcastID uint16
}

func newDescriptorDataBroadcastID(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &DataBroadcastID{
		Header: h,
	}
	dd = d

	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d.DataBroadcastID = binary.BigEndian.Uint16(bs)

	if d.Selector, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *DataBroadcastID) CalcLength() int {
	return 2 + len(d.Selector)
}

func (d *DataBroadcastID) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, byte(d.DataBroadcastID>>8), byte(d.DataBroadcastID))
	return append(dst, d.Selector...)
}
