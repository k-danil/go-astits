package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// CellFrequencyLink represents a cell frequency link descriptor: the cells of a
// terrestrial network and the frequencies used in them and their subcells.
// Chapter: 6.2.6 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type CellFrequencyLink struct {
	Cells  []CellFrequencyLinkCell `json:"_cells"`
	Header Header                  `json:"_header"`
}

// CellFrequencyLinkCell is one cell of a cell frequency link descriptor
type CellFrequencyLinkCell struct {
	Subcells  []CellFrequencyLinkSubcell `json:"_subcells"`
	CellID    uint16                     `json:"cell_id"`
	Frequency uint32                     `json:"frequency"`
}

// CellFrequencyLinkSubcell is one subcell of a cell frequency link cell
type CellFrequencyLinkSubcell struct {
	CellIDExtension     uint8  `json:"cell_id_extension"`
	TransposerFrequency uint32 `json:"transposer_frequency"`
}

func newDescriptorCellFrequencyLink(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &CellFrequencyLink{
		Header: h,
	}
	dd = d

	for i.Offset() < offsetEnd {
		var cell CellFrequencyLinkCell
		var bs []byte
		if bs, err = i.NextBytesNoCopy(7); err != nil || len(bs) < 7 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		cell.CellID = binary.BigEndian.Uint16(bs[0:2])
		cell.Frequency = binary.BigEndian.Uint32(bs[2:6])
		subEnd := i.Offset() + int(bs[6])

		for i.Offset() < subEnd {
			var sub CellFrequencyLinkSubcell
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

func (d *CellFrequencyLink) CalcLength() (n int) {
	for _, cell := range d.Cells {
		n += 7 + 5*len(cell.Subcells)
	}
	return
}

func (d *CellFrequencyLink) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	for _, cell := range d.Cells {
		dst = append(dst,
			byte(cell.CellID>>8), byte(cell.CellID),
			byte(cell.Frequency>>24), byte(cell.Frequency>>16), byte(cell.Frequency>>8), byte(cell.Frequency),
			uint8(5*len(cell.Subcells)))
		for _, sub := range cell.Subcells {
			dst = append(dst, sub.CellIDExtension,
				byte(sub.TransposerFrequency>>24), byte(sub.TransposerFrequency>>16),
				byte(sub.TransposerFrequency>>8), byte(sub.TransposerFrequency))
		}
	}
	return dst
}
