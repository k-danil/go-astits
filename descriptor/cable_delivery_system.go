package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

// Frequency and SymbolRate stay BCD-packed as on the wire (XXX.XXXX MHz / Msymbol per second).
type CableDeliverySystem struct {
	Header     Header `json:"_header"`
	Frequency  uint32 `json:"frequency"`
	SymbolRate uint32 `json:"symbol_rate"`
	FECOuter   uint8  `json:"FEC_outer"`
	Modulation uint8  `json:"modulation"`
	FECInner   uint8  `json:"FEC_inner"`
}

func newDescriptorCableDeliverySystem(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	d := &CableDeliverySystem{
		Header: h,
	}
	dd = d

	var bs []byte
	if bs, err = i.NextBytesNoCopy(11); err != nil || len(bs) < 11 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d.Frequency = binary.BigEndian.Uint32(bs[0:4])
	d.FECOuter = bs[5] & 0x0f
	d.Modulation = bs[6]
	d.SymbolRate = uint32(bs[7])<<20 | uint32(bs[8])<<12 | uint32(bs[9])<<4 | uint32(bs[10])>>4
	d.FECInner = bs[10] & 0x0f
	return
}

func (d *CableDeliverySystem) CalcLength() int {
	return 11
}

func (d *CableDeliverySystem) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, byte(d.Frequency>>24), byte(d.Frequency>>16), byte(d.Frequency>>8), byte(d.Frequency))
	dst = append(dst, 0xff, 0xf0|d.FECOuter&0x0f, d.Modulation)
	return append(dst,
		byte(d.SymbolRate>>20), byte(d.SymbolRate>>12), byte(d.SymbolRate>>4),
		byte(d.SymbolRate<<4)|d.FECInner&0x0f)
}
