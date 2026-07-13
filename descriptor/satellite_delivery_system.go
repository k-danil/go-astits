package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// SatelliteDeliverySystem represents a satellite delivery system descriptor.
// Frequency (GHz), OrbitalPosition (degrees) and SymbolRate keep their raw
// BCD-packed values; RollOff is only meaningful when ModulationSystem (DVB-S2).
// Chapter: 6.2.13.2 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type SatelliteDeliverySystem struct {
	Header           Header `json:"_header"`
	Frequency        uint32 `json:"frequency"`
	SymbolRate       uint32 `json:"symbol_rate"`
	OrbitalPosition  uint16 `json:"orbital_position"`
	Polarization     uint8  `json:"polarization"`
	RollOff          uint8  `json:"roll_off"`
	ModulationType   uint8  `json:"modulation_type"`
	FECInner         uint8  `json:"FEC_inner"`
	WestEastFlag     bool   `json:"west_east_flag"`
	ModulationSystem bool   `json:"modulation_system"`
}

func newDescriptorSatelliteDeliverySystem(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	d := &SatelliteDeliverySystem{
		Header: h,
	}
	dd = d

	var bs []byte
	if bs, err = i.NextBytesNoCopy(11); err != nil || len(bs) < 11 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d.Frequency = binary.BigEndian.Uint32(bs[0:4])
	d.OrbitalPosition = binary.BigEndian.Uint16(bs[4:6])
	d.WestEastFlag = bs[6]&0x80 > 0
	d.Polarization = bs[6] >> 5 & 0x03
	d.RollOff = bs[6] >> 3 & 0x03
	d.ModulationSystem = bs[6]&0x04 > 0
	d.ModulationType = bs[6] & 0x03
	d.SymbolRate = uint32(bs[7])<<20 | uint32(bs[8])<<12 | uint32(bs[9])<<4 | uint32(bs[10])>>4
	d.FECInner = bs[10] & 0x0f
	return
}

func (d *SatelliteDeliverySystem) CalcLength() int {
	return 11
}

func (d *SatelliteDeliverySystem) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, byte(d.Frequency>>24), byte(d.Frequency>>16), byte(d.Frequency>>8), byte(d.Frequency))
	dst = append(dst, byte(d.OrbitalPosition>>8), byte(d.OrbitalPosition))
	b := d.Polarization&0x03<<5 | d.RollOff&0x03<<3 | d.ModulationType&0x03
	if d.WestEastFlag {
		b |= 0x80
	}
	if d.ModulationSystem {
		b |= 0x04
	}
	dst = append(dst, b)
	return append(dst,
		byte(d.SymbolRate>>20), byte(d.SymbolRate>>12), byte(d.SymbolRate>>4),
		byte(d.SymbolRate<<4)|d.FECInner&0x0f)
}
