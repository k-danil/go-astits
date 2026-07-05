package psi

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/ts"
)

const (
	patSectionEntryBytesSize = 4 // 16 bits + 3 reserved + 13 bits = 32 bits
)

// PAT represents a PAT data
// https://en.wikipedia.org/wiki/Program-specific_information
type PAT struct {
	Programs          []PATProgram
	TransportStreamID uint16
}

// PATProgram represents a PAT program
type PATProgram struct {
	ProgramMapID  uint16 // The packet identifier that contains the associated PMT
	ProgramNumber uint16 // Relates to the Table ID extension in the associated PMT. A value of 0 is reserved for a NIT packet identifier.
}

// parsePATSection parses a PAT section
func parsePATSection(i *bytesiter.Iterator, offsetSectionsEnd int, tableIDExtension uint16) (d *PAT, err error) {
	// The syntax header may have overrun a lying section length
	n := (offsetSectionsEnd - i.Offset()) / patSectionEntryBytesSize
	if n < 0 {
		return nil, fmt.Errorf("astits: PAT section end %d is before its start: %w", offsetSectionsEnd, ts.ErrInvalidData)
	}

	d = &PAT{
		TransportStreamID: tableIDExtension,
		Programs:          make([]PATProgram, n),
	}

	for idx := range d.Programs {
		var bs []byte
		if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		val := binary.BigEndian.Uint32(bs)
		d.Programs[idx] = PATProgram{
			ProgramMapID:  uint16(val & 0x1fff),
			ProgramNumber: uint16(val >> 16),
		}
	}
	return
}

func (d *PAT) CalcSectionLength() int {
	return 4 * len(d.Programs)
}

func (d *PAT) appendSection(dst []byte) []byte {
	for _, p := range d.Programs {
		dst = append(dst,
			byte(p.ProgramNumber>>8), byte(p.ProgramNumber),
			0xe0|byte(p.ProgramMapID>>8)&0x1f, byte(p.ProgramMapID))
	}
	return dst
}
