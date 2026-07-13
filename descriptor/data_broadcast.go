package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// DataBroadcast represents a data broadcast descriptor: the type of a data
// component plus an optional text description.
// Chapter: 6.2.11 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DataBroadcast struct {
	Selector        []byte  `json:"selector_byte"`
	Text            []byte  `json:"text_char"`
	Header          Header  `json:"_header"`
	DataBroadcastID uint16  `json:"data_broadcast_id"`
	Language        [3]byte `json:"ISO_639_language_code"`
	ComponentTag    uint8   `json:"component_tag"`
}

func newDescriptorDataBroadcast(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	d := &DataBroadcast{
		Header: h,
	}
	dd = d

	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d.DataBroadcastID = binary.BigEndian.Uint16(bs)

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.ComponentTag = b

	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	if d.Selector, err = i.NextBytes(int(b)); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	copy(d.Language[:], bs)

	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	if d.Text, err = i.NextBytes(int(b)); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *DataBroadcast) CalcLength() int {
	return 8 + len(d.Selector) + len(d.Text)
}

func (d *DataBroadcast) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, byte(d.DataBroadcastID>>8), byte(d.DataBroadcastID), d.ComponentTag)
	dst = append(dst, uint8(len(d.Selector)))
	dst = append(dst, d.Selector...)
	dst = append(dst, d.Language[:]...)
	dst = append(dst, uint8(len(d.Text)))
	return append(dst, d.Text...)
}
