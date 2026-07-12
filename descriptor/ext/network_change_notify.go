package ext

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// NetworkChangeNotify represents a network change notify extension
// descriptor: scheduled network-change events, grouped by cell.
// Chapter: 6.4.8 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type NetworkChangeNotify struct {
	Cells []NetworkChangeCell
}

// NetworkChangeCell groups the network changes signalled for one cell
type NetworkChangeCell struct {
	Changes []NetworkChange
	CellID  uint16
}

// NetworkChange is one scheduled network-change event. StartTimeOfChange
// (40-bit MJD+BCD) and ChangeDuration (24-bit BCD) keep their raw values; the
// invariant-TS ids are present only when InvariantTSPresent.
type NetworkChange struct {
	StartTimeOfChange    uint64
	ChangeDuration       uint32
	InvariantTSTSID      uint16
	InvariantTSONID      uint16
	NetworkChangeID      uint8
	NetworkChangeVersion uint8
	ReceiverCategory     uint8
	ChangeType           uint8
	MessageID            uint8
	InvariantTSPresent   bool
}

func parseNetworkChangeNotify(i *bytesiter.Iterator, offsetEnd int) (d *NetworkChangeNotify, err error) {
	d = &NetworkChangeNotify{}

	for i.Offset() < offsetEnd {
		var cell NetworkChangeCell
		var bs []byte
		if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		cell.CellID = binary.BigEndian.Uint16(bs[0:2])
		loopEnd := i.Offset() + int(bs[2])

		for i.Offset() < loopEnd {
			var c NetworkChange
			if bs, err = i.NextBytesNoCopy(12); err != nil || len(bs) < 12 {
				err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
				return
			}
			c.NetworkChangeID = bs[0]
			c.NetworkChangeVersion = bs[1]
			c.StartTimeOfChange = uint64(bs[2])<<32 | uint64(bs[3])<<24 |
				uint64(bs[4])<<16 | uint64(bs[5])<<8 | uint64(bs[6])
			c.ChangeDuration = uint32(bs[7])<<16 | uint32(bs[8])<<8 | uint32(bs[9])
			c.ReceiverCategory = bs[10] >> 5 & 0x07
			c.InvariantTSPresent = bs[10]&0x10 > 0
			c.ChangeType = bs[10] & 0x0f
			c.MessageID = bs[11]

			if c.InvariantTSPresent {
				if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
					err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
					return
				}
				c.InvariantTSTSID = binary.BigEndian.Uint16(bs[0:2])
				c.InvariantTSONID = binary.BigEndian.Uint16(bs[2:4])
			}
			cell.Changes = append(cell.Changes, c)
		}
		d.Cells = append(d.Cells, cell)
	}
	return
}

func (c *NetworkChange) length() int {
	if c.InvariantTSPresent {
		return 16
	}
	return 12
}

func (d *NetworkChangeNotify) CalcLength() (n int) {
	for idx := range d.Cells {
		n += 3
		for ci := range d.Cells[idx].Changes {
			n += d.Cells[idx].Changes[ci].length()
		}
	}
	return
}

func (c *NetworkChange) append(dst []byte) []byte {
	dst = append(dst, c.NetworkChangeID, c.NetworkChangeVersion,
		byte(c.StartTimeOfChange>>32), byte(c.StartTimeOfChange>>24), byte(c.StartTimeOfChange>>16),
		byte(c.StartTimeOfChange>>8), byte(c.StartTimeOfChange),
		byte(c.ChangeDuration>>16), byte(c.ChangeDuration>>8), byte(c.ChangeDuration),
		c.ReceiverCategory&0x07<<5|invariantBit(c.InvariantTSPresent)|c.ChangeType&0x0f,
		c.MessageID)
	if c.InvariantTSPresent {
		dst = append(dst, byte(c.InvariantTSTSID>>8), byte(c.InvariantTSTSID),
			byte(c.InvariantTSONID>>8), byte(c.InvariantTSONID))
	}
	return dst
}

func invariantBit(present bool) byte {
	if present {
		return 0x10
	}
	return 0
}

func (d *NetworkChangeNotify) Append(dst []byte) []byte {
	for idx := range d.Cells {
		cell := &d.Cells[idx]
		var loopLength int
		for ci := range cell.Changes {
			loopLength += cell.Changes[ci].length()
		}
		dst = append(dst, byte(cell.CellID>>8), byte(cell.CellID), uint8(loopLength))
		for ci := range cell.Changes {
			dst = cell.Changes[ci].append(dst)
		}
	}
	return dst
}
