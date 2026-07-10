package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// Mosaic represents a mosaic descriptor: how a mosaic video component is
// partitioned into cells and what each logical cell links to (bouquet, service,
// event, …) keyed by CellLinkageInfo.
// Chapter: 6.2.21 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type Mosaic struct {
	Cells                             []MosaicCell
	Header                            Header
	NumberOfHorizontalElementaryCells uint8
	NumberOfVerticalElementaryCells   uint8
	MosaicEntryPoint                  bool
}

// MosaicCell is one logical cell of a mosaic descriptor. The linkage ids are
// present according to CellLinkageInfo (1: bouquet; 2/3: service; 4: event).
type MosaicCell struct {
	ElementaryCellIDs           []uint8
	OriginalNetworkID           uint16
	TransportStreamID           uint16
	ServiceID                   uint16
	EventID                     uint16
	BouquetID                   uint16
	LogicalCellID               uint8
	LogicalCellPresentationInfo uint8
	CellLinkageInfo             uint8
}

func newDescriptorMosaic(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &Mosaic{
		Header: h,
	}
	dd = d

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.MosaicEntryPoint = b&0x80 > 0
	d.NumberOfHorizontalElementaryCells = b >> 4 & 0x07
	d.NumberOfVerticalElementaryCells = b & 0x07

	for i.Offset() < offsetEnd {
		var cell MosaicCell
		var bs []byte
		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		cell.LogicalCellID = bs[0] >> 2 & 0x3f
		cell.LogicalCellPresentationInfo = bs[1] & 0x07

		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		cell.ElementaryCellIDs = make([]uint8, int(b))
		for idx := range cell.ElementaryCellIDs {
			if b, err = i.NextByte(); err != nil {
				err = fmt.Errorf("astits: fetching next byte failed: %w", err)
				return
			}
			cell.ElementaryCellIDs[idx] = b & 0x3f
		}

		if cell.CellLinkageInfo, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		if err = readMosaicLinkage(i, &cell); err != nil {
			return
		}
		d.Cells = append(d.Cells, cell)
	}
	return
}

func readMosaicLinkage(i *bytesiter.Iterator, cell *MosaicCell) (err error) {
	n := mosaicLinkageLength(cell.CellLinkageInfo)
	if n == 0 {
		return
	}
	var bs []byte
	if bs, err = i.NextBytesNoCopy(n); err != nil || len(bs) < n {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	switch cell.CellLinkageInfo {
	case 0x01:
		cell.BouquetID = binary.BigEndian.Uint16(bs[0:2])
	case 0x02, 0x03:
		cell.OriginalNetworkID = binary.BigEndian.Uint16(bs[0:2])
		cell.TransportStreamID = binary.BigEndian.Uint16(bs[2:4])
		cell.ServiceID = binary.BigEndian.Uint16(bs[4:6])
	case 0x04:
		cell.OriginalNetworkID = binary.BigEndian.Uint16(bs[0:2])
		cell.TransportStreamID = binary.BigEndian.Uint16(bs[2:4])
		cell.ServiceID = binary.BigEndian.Uint16(bs[4:6])
		cell.EventID = binary.BigEndian.Uint16(bs[6:8])
	}
	return
}

func mosaicLinkageLength(cellLinkageInfo uint8) int {
	switch cellLinkageInfo {
	case 0x01:
		return 2
	case 0x02, 0x03:
		return 6
	case 0x04:
		return 8
	}
	return 0
}

func (d *Mosaic) CalcLength() (n int) {
	n = 1
	for _, cell := range d.Cells {
		n += 4 + len(cell.ElementaryCellIDs) + mosaicLinkageLength(cell.CellLinkageInfo)
	}
	return
}

func (d *Mosaic) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	b := byte(0x08) | d.NumberOfHorizontalElementaryCells&0x07<<4 | d.NumberOfVerticalElementaryCells&0x07
	if d.MosaicEntryPoint {
		b |= 0x80
	}
	dst = append(dst, b)

	for _, cell := range d.Cells {
		dst = append(dst, cell.LogicalCellID&0x3f<<2|0x03, 0xf8|cell.LogicalCellPresentationInfo&0x07)
		dst = append(dst, uint8(len(cell.ElementaryCellIDs)))
		for _, id := range cell.ElementaryCellIDs {
			dst = append(dst, 0xc0|id&0x3f)
		}
		dst = append(dst, cell.CellLinkageInfo)
		switch cell.CellLinkageInfo {
		case 0x01:
			dst = append(dst, byte(cell.BouquetID>>8), byte(cell.BouquetID))
		case 0x02, 0x03:
			dst = append(dst, byte(cell.OriginalNetworkID>>8), byte(cell.OriginalNetworkID),
				byte(cell.TransportStreamID>>8), byte(cell.TransportStreamID),
				byte(cell.ServiceID>>8), byte(cell.ServiceID))
		case 0x04:
			dst = append(dst, byte(cell.OriginalNetworkID>>8), byte(cell.OriginalNetworkID),
				byte(cell.TransportStreamID>>8), byte(cell.TransportStreamID),
				byte(cell.ServiceID>>8), byte(cell.ServiceID),
				byte(cell.EventID>>8), byte(cell.EventID))
		}
	}
	return dst
}
