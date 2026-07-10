package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// CellList represents a cell list descriptor: the cells of a terrestrial
// network and their coverage areas (raw latitude/longitude and extent fields).
// Chapter: 6.2.7 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type CellList struct {
	Cells  []CellListCell
	Header Header
}

// CellListCell is one cell of a cell list descriptor
type CellListCell struct {
	Subcells              []CellListSubcell
	CellID                uint16
	CellLatitude          uint16
	CellLongitude         uint16
	CellExtentOfLatitude  uint16
	CellExtentOfLongitude uint16
}

// CellListSubcell is one subcell of a cell list cell
type CellListSubcell struct {
	SubcellLatitude          uint16
	SubcellLongitude         uint16
	SubcellExtentOfLatitude  uint16
	SubcellExtentOfLongitude uint16
	CellIDExtension          uint8
}

func newDescriptorCellList(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &CellList{
		Header: h,
	}
	dd = d

	for i.Offset() < offsetEnd {
		var cell CellListCell
		var bs []byte
		if bs, err = i.NextBytesNoCopy(10); err != nil || len(bs) < 10 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		cell.CellID = binary.BigEndian.Uint16(bs[0:2])
		cell.CellLatitude = binary.BigEndian.Uint16(bs[2:4])
		cell.CellLongitude = binary.BigEndian.Uint16(bs[4:6])
		cell.CellExtentOfLatitude = uint16(bs[6])<<4 | uint16(bs[7])>>4
		cell.CellExtentOfLongitude = uint16(bs[7]&0x0f)<<8 | uint16(bs[8])
		subEnd := i.Offset() + int(bs[9])

		for i.Offset() < subEnd {
			var sub CellListSubcell
			if bs, err = i.NextBytesNoCopy(8); err != nil || len(bs) < 8 {
				err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
				return
			}
			sub.CellIDExtension = bs[0]
			sub.SubcellLatitude = binary.BigEndian.Uint16(bs[1:3])
			sub.SubcellLongitude = binary.BigEndian.Uint16(bs[3:5])
			sub.SubcellExtentOfLatitude = uint16(bs[5])<<4 | uint16(bs[6])>>4
			sub.SubcellExtentOfLongitude = uint16(bs[6]&0x0f)<<8 | uint16(bs[7])
			cell.Subcells = append(cell.Subcells, sub)
		}
		d.Cells = append(d.Cells, cell)
	}
	return
}

func (d *CellList) CalcLength() (n int) {
	for _, cell := range d.Cells {
		n += 10 + 8*len(cell.Subcells)
	}
	return
}

func appendExtents(dst []byte, extLat, extLon uint16) []byte {
	return append(dst,
		byte(extLat>>4),
		byte(extLat&0x0f)<<4|byte(extLon>>8&0x0f),
		byte(extLon))
}

func (d *CellList) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	for _, cell := range d.Cells {
		dst = append(dst,
			byte(cell.CellID>>8), byte(cell.CellID),
			byte(cell.CellLatitude>>8), byte(cell.CellLatitude),
			byte(cell.CellLongitude>>8), byte(cell.CellLongitude))
		dst = appendExtents(dst, cell.CellExtentOfLatitude, cell.CellExtentOfLongitude)
		dst = append(dst, uint8(8*len(cell.Subcells)))
		for _, sub := range cell.Subcells {
			dst = append(dst, sub.CellIDExtension,
				byte(sub.SubcellLatitude>>8), byte(sub.SubcellLatitude),
				byte(sub.SubcellLongitude>>8), byte(sub.SubcellLongitude))
			dst = appendExtents(dst, sub.SubcellExtentOfLatitude, sub.SubcellExtentOfLongitude)
		}
	}
	return dst
}
