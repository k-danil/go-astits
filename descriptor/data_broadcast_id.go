package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type DataBroadcastID struct {
	Selector        []byte `json:"id_selector_byte"`
	Header          Header `json:"_header"`
	DataBroadcastID uint16 `json:"data_broadcast_id"`
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
