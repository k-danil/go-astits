package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type CellList struct {
	Cells  []CellListCell `json:"_cells"`
	Header Header         `json:"_header"`
}

// Latitudes and longitudes are two's complement (south and west negative), though the syntax table calls them uimsbf.
type CellListCell struct {
	Subcells              []CellListSubcell `json:"_subcells"`
	CellID                uint16            `json:"cell_id"`
	CellLatitude          int16             `json:"cell_latitude"`
	CellLongitude         int16             `json:"cell_longitude"`
	CellExtentOfLatitude  uint16            `json:"cell_extent_of_latitude"`
	CellExtentOfLongitude uint16            `json:"cell_extent_of_longitude"`
}

type CellListSubcell struct {
	SubcellLatitude          int16  `json:"subcell_latitude"`
	SubcellLongitude         int16  `json:"subcell_longitude"`
	SubcellExtentOfLatitude  uint16 `json:"subcell_extent_of_latitude"`
	SubcellExtentOfLongitude uint16 `json:"subcell_extent_of_longitude"`
	CellIDExtension          uint8  `json:"cell_id_extension"`
}

func newDescriptorCellList(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &CellList{
		Header: h,
	}
	dd = d

	for i.Offset() < offsetEnd {
		var cell CellListCell
		var bs []byte
		if bs, err = i.NextBytesNoCopy(cellListCellSize); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		cell.CellID = binary.BigEndian.Uint16(bs[0:2])
		cell.CellLatitude = int16(binary.BigEndian.Uint16(bs[2:4]))
		cell.CellLongitude = int16(binary.BigEndian.Uint16(bs[4:6]))
		cell.CellExtentOfLatitude = uint16(bs[6])<<4 | uint16(bs[7])>>4
		cell.CellExtentOfLongitude = uint16(bs[7]&0x0f)<<8 | uint16(bs[8])
		subEnd := i.Offset() + int(bs[9])

		for i.Offset() < subEnd {
			var sub CellListSubcell
			if bs, err = i.NextBytesNoCopy(cellListSubcellSize); err != nil {
				err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
				return
			}
			sub.CellIDExtension = bs[0]
			sub.SubcellLatitude = int16(binary.BigEndian.Uint16(bs[1:3]))
			sub.SubcellLongitude = int16(binary.BigEndian.Uint16(bs[3:5]))
			sub.SubcellExtentOfLatitude = uint16(bs[5])<<4 | uint16(bs[6])>>4
			sub.SubcellExtentOfLongitude = uint16(bs[6]&0x0f)<<8 | uint16(bs[7])
			cell.Subcells = append(cell.Subcells, sub)
		}
		d.Cells = append(d.Cells, cell)
	}
	return
}

const (
	cellListCellSize    = 10
	cellListSubcellSize = 8
)

func (d *CellList) CalcLength() (n int) {
	for _, cell := range d.Cells {
		n += cellListCellSize + cellListSubcellSize*len(cell.Subcells)
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
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	for _, cell := range d.Cells {
		dst = append(dst,
			byte(cell.CellID>>8), byte(cell.CellID),
			byte(cell.CellLatitude>>8), byte(cell.CellLatitude),
			byte(cell.CellLongitude>>8), byte(cell.CellLongitude))
		dst = appendExtents(dst, cell.CellExtentOfLatitude, cell.CellExtentOfLongitude)
		dst = append(dst, uint8(cellListSubcellSize*len(cell.Subcells)))
		for _, sub := range cell.Subcells {
			dst = append(dst, sub.CellIDExtension,
				byte(sub.SubcellLatitude>>8), byte(sub.SubcellLatitude),
				byte(sub.SubcellLongitude>>8), byte(sub.SubcellLongitude))
			dst = appendExtents(dst, sub.SubcellExtentOfLatitude, sub.SubcellExtentOfLongitude)
		}
	}
	return dst
}
