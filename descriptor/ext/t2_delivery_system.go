package ext

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// T2DeliverySystem represents a T2 delivery system extension
// descriptor: the DVB-T2 tuning parameters mapping a transport stream to a data
// PLP. The block after T2SystemID is present only when HasExtension.
// Chapter: 6.4.5.3 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type T2DeliverySystem struct {
	Cells              []T2Cell `json:"_cells"`
	T2SystemID         uint16   `json:"T2_system_id"`
	PLPID              uint8    `json:"plp_id"`
	SISOMISO           uint8    `json:"SISO/MISO"`
	Bandwidth          uint8    `json:"bandwidth"`
	GuardInterval      uint8    `json:"guard_interval"`
	TransmissionMode   uint8    `json:"transmission_mode"`
	OtherFrequencyFlag bool     `json:"other_frequency_flag"`
	TFSFlag            bool     `json:"tfs_flag"`
	HasExtension       bool     `json:"_has_extension"`
}

// T2Cell is one cell of a T2 delivery system descriptor. CentreFrequencies
// holds one frequency, or several when the descriptor's TFSFlag is set.
type T2Cell struct {
	CentreFrequencies []uint32    `json:"_centre_frequencies"`
	Subcells          []T2Subcell `json:"_subcells"`
	CellID            uint16      `json:"cell_id"`
}

// T2Subcell is one subcell of a T2 cell
type T2Subcell struct {
	CellIDExtension     uint8  `json:"cell_id_extension"`
	TransposerFrequency uint32 `json:"transposer_frequency"`
}

func parseT2DeliverySystem(i *bytesiter.Iterator, offsetEnd int) (d *T2DeliverySystem, err error) {
	d = &T2DeliverySystem{}

	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d.PLPID = bs[0]
	d.T2SystemID = binary.BigEndian.Uint16(bs[1:3])

	if i.Offset() >= offsetEnd {
		return
	}
	d.HasExtension = true

	if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d.SISOMISO = bs[0] >> 6 & 0x03
	d.Bandwidth = bs[0] >> 2 & 0x0f
	d.GuardInterval = bs[1] >> 5 & 0x07
	d.TransmissionMode = bs[1] >> 2 & 0x07
	d.OtherFrequencyFlag = bs[1]&0x02 > 0
	d.TFSFlag = bs[1]&0x01 > 0

	for i.Offset() < offsetEnd {
		var cell T2Cell
		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		cell.CellID = binary.BigEndian.Uint16(bs)

		if d.TFSFlag {
			var b byte
			if b, err = i.NextByte(); err != nil {
				err = fmt.Errorf("astits: fetching next byte failed: %w", err)
				return
			}
			freqEnd := i.Offset() + int(b)
			for i.Offset() < freqEnd {
				var freq uint32
				if freq, err = nextUint32(i); err != nil {
					return
				}
				cell.CentreFrequencies = append(cell.CentreFrequencies, freq)
			}
		} else {
			var freq uint32
			if freq, err = nextUint32(i); err != nil {
				return
			}
			cell.CentreFrequencies = append(cell.CentreFrequencies, freq)
		}

		var b byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		subEnd := i.Offset() + int(b)
		for i.Offset() < subEnd {
			var sub T2Subcell
			if bs, err = i.NextBytesNoCopy(5); err != nil || len(bs) < 5 {
				err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
				return
			}
			sub.CellIDExtension = bs[0]
			sub.TransposerFrequency = binary.BigEndian.Uint32(bs[1:5])
			cell.Subcells = append(cell.Subcells, sub)
		}
		d.Cells = append(d.Cells, cell)
	}
	return
}

func nextUint32(i *bytesiter.Iterator) (v uint32, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return binary.BigEndian.Uint32(bs), nil
}

func (d *T2DeliverySystem) CalcLength() (n int) {
	n = 3
	if !d.HasExtension {
		return
	}
	n += 2
	for idx := range d.Cells {
		cell := &d.Cells[idx]
		n += 2
		if d.TFSFlag {
			n += 1 + 4*len(cell.CentreFrequencies)
		} else {
			n += 4
		}
		n += 1 + 5*len(cell.Subcells)
	}
	return
}

func (d *T2DeliverySystem) Append(dst []byte) []byte {
	dst = append(dst, d.PLPID, byte(d.T2SystemID>>8), byte(d.T2SystemID))
	if !d.HasExtension {
		return dst
	}

	dst = append(dst, d.SISOMISO&0x03<<6|d.Bandwidth&0x0f<<2|0x03)
	b := d.GuardInterval&0x07<<5 | d.TransmissionMode&0x07<<2
	if d.OtherFrequencyFlag {
		b |= 0x02
	}
	if d.TFSFlag {
		b |= 0x01
	}
	dst = append(dst, b)

	for idx := range d.Cells {
		cell := &d.Cells[idx]
		dst = append(dst, byte(cell.CellID>>8), byte(cell.CellID))
		if d.TFSFlag {
			dst = append(dst, uint8(4*len(cell.CentreFrequencies)))
		}
		for _, f := range cell.CentreFrequencies {
			dst = append(dst, byte(f>>24), byte(f>>16), byte(f>>8), byte(f))
		}
		dst = append(dst, uint8(5*len(cell.Subcells)))
		for _, sub := range cell.Subcells {
			dst = append(dst, sub.CellIDExtension,
				byte(sub.TransposerFrequency>>24), byte(sub.TransposerFrequency>>16),
				byte(sub.TransposerFrequency>>8), byte(sub.TransposerFrequency))
		}
	}
	return dst
}
