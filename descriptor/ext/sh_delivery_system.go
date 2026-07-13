package ext

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// SHDeliverySystem represents an SH delivery system extension
// descriptor: the DVB-SH physical parameters — a diversity mode plus a loop of
// modulation elements, each either TDM (ModulationType 0) or OFDM
// (ModulationType 1) and optionally an interleaver block.
// Chapter: 6.4.5.2 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type SHDeliverySystem struct {
	Modulations   []SHModulation `json:"_modulations"`
	DiversityMode uint8          `json:"diversity_mode"`
}

// SHModulation is one modulation element of an SH delivery system descriptor.
// Which of the TDM / OFDM fields are meaningful is selected by ModulationType;
// the interleaver fields are present only when InterleaverPresence.
type SHModulation struct {
	ModulationType      uint8 `json:"modulation_type"`
	InterleaverPresence bool  `json:"interleaver_presence"`
	InterleaverType     bool  `json:"interleaver_type"`

	// TDM (ModulationType == 0)
	Polarization   uint8 `json:"polarization"`
	RollOff        uint8 `json:"roll_off"`
	ModulationMode uint8 `json:"modulation_mode"`
	SymbolRate     uint8 `json:"symbol_rate"`

	// OFDM (ModulationType == 1)
	Bandwidth                 uint8 `json:"bandwidth"`
	ConstellationAndHierarchy uint8 `json:"constellation_and_hierarchy"`
	GuardInterval             uint8 `json:"guard_interval"`
	TransmissionMode          uint8 `json:"transmission_mode"`
	Priority                  bool  `json:"priority"`
	CommonFrequency           bool  `json:"common_frequency"`

	// TDM and OFDM
	CodeRate uint8 `json:"code_rate"`

	// interleaver (InterleaverPresence)
	CommonMultiplier  uint8 `json:"common_multiplier"`
	NofLateTaps       uint8 `json:"nof_late_taps"`
	NofSlices         uint8 `json:"nof_slices"`
	SliceDistance     uint8 `json:"slice_distance"`
	NonLateIncrements uint8 `json:"non_late_increments"`
}

func parseSHDeliverySystem(i *bytesiter.Iterator, offsetEnd int) (d *SHDeliverySystem, err error) {
	d = &SHDeliverySystem{}

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.DiversityMode = b >> 4 & 0x0f

	for i.Offset() < offsetEnd {
		var m SHModulation
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		m.ModulationType = b >> 7 & 0x01
		m.InterleaverPresence = b&0x40 > 0
		m.InterleaverType = b&0x20 > 0

		var bs []byte
		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		val := binary.BigEndian.Uint16(bs)
		if m.ModulationType == 0 {
			m.Polarization = uint8(val >> 14 & 0x03)
			m.RollOff = uint8(val >> 12 & 0x03)
			m.ModulationMode = uint8(val >> 10 & 0x03)
			m.CodeRate = uint8(val >> 6 & 0x0f)
			m.SymbolRate = uint8(val >> 1 & 0x1f)
		} else {
			m.Bandwidth = uint8(val >> 13 & 0x07)
			m.Priority = val&0x1000 > 0
			m.ConstellationAndHierarchy = uint8(val >> 9 & 0x07)
			m.CodeRate = uint8(val >> 5 & 0x0f)
			m.GuardInterval = uint8(val >> 3 & 0x03)
			m.TransmissionMode = uint8(val >> 1 & 0x03)
			m.CommonFrequency = val&0x01 > 0
		}

		if m.InterleaverPresence {
			if !m.InterleaverType {
				if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
					err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
					return
				}
				v := binary.BigEndian.Uint32(bs)
				m.CommonMultiplier = uint8(v >> 26 & 0x3f)
				m.NofLateTaps = uint8(v >> 20 & 0x3f)
				m.NofSlices = uint8(v >> 14 & 0x3f)
				m.SliceDistance = uint8(v >> 6 & 0xff)
				m.NonLateIncrements = uint8(v & 0x3f)
			} else {
				if b, err = i.NextByte(); err != nil {
					err = fmt.Errorf("astits: fetching next byte failed: %w", err)
					return
				}
				m.CommonMultiplier = b >> 2 & 0x3f
			}
		}
		d.Modulations = append(d.Modulations, m)
	}
	return
}

func (m *SHModulation) length() int {
	n := 3
	if m.InterleaverPresence {
		if !m.InterleaverType {
			n += 4
		} else {
			n++
		}
	}
	return n
}

func (d *SHDeliverySystem) CalcLength() (n int) {
	n = 1
	for idx := range d.Modulations {
		n += d.Modulations[idx].length()
	}
	return
}

func (d *SHDeliverySystem) Append(dst []byte) []byte {
	dst = append(dst, d.DiversityMode&0x0f<<4|0x0f)
	for idx := range d.Modulations {
		m := &d.Modulations[idx]
		b := m.ModulationType&0x01<<7 | 0x1f
		if m.InterleaverPresence {
			b |= 0x40
		}
		if m.InterleaverType {
			b |= 0x20
		}
		dst = append(dst, b)

		var val uint16
		if m.ModulationType == 0 {
			val = uint16(m.Polarization&0x03)<<14 | uint16(m.RollOff&0x03)<<12 |
				uint16(m.ModulationMode&0x03)<<10 | uint16(m.CodeRate&0x0f)<<6 |
				uint16(m.SymbolRate&0x1f)<<1 | 0x01
		} else {
			val = uint16(m.Bandwidth&0x07)<<13 | uint16(m.ConstellationAndHierarchy&0x07)<<9 |
				uint16(m.CodeRate&0x0f)<<5 | uint16(m.GuardInterval&0x03)<<3 |
				uint16(m.TransmissionMode&0x03)<<1
			if m.Priority {
				val |= 0x1000
			}
			if m.CommonFrequency {
				val |= 0x01
			}
		}
		dst = append(dst, byte(val>>8), byte(val))

		if m.InterleaverPresence {
			if !m.InterleaverType {
				v := uint32(m.CommonMultiplier&0x3f)<<26 | uint32(m.NofLateTaps&0x3f)<<20 |
					uint32(m.NofSlices&0x3f)<<14 | uint32(m.SliceDistance)<<6 |
					uint32(m.NonLateIncrements&0x3f)
				dst = append(dst, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
			} else {
				dst = append(dst, m.CommonMultiplier&0x3f<<2|0x03)
			}
		}
	}
	return dst
}
