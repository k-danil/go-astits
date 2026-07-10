package psi

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// RST represents an RST: the running status table signals rapid updates to the
// running status of events. It has neither a syntax header nor a CRC.
// Page: 38 | Chapter: 5.2.6 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type RST struct {
	Events []RSTEvent
}

// RSTEvent represents one running-status entry of an RST
type RSTEvent struct {
	TransportStreamID uint16
	OriginalNetworkID uint16
	ServiceID         uint16
	EventID           uint16
	RunningStatus     uint8
}

// parseRSTSection parses an RST section
func parseRSTSection(i *bytesiter.Iterator, offsetSectionsEnd int) (d *RST, err error) {
	d = &RST{}

	for i.Offset() < offsetSectionsEnd {
		e := RSTEvent{}

		var bs []byte
		if bs, err = i.NextBytesNoCopy(8); err != nil || len(bs) < 8 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		e.TransportStreamID = binary.BigEndian.Uint16(bs[0:2])
		e.OriginalNetworkID = binary.BigEndian.Uint16(bs[2:4])
		e.ServiceID = binary.BigEndian.Uint16(bs[4:6])
		e.EventID = binary.BigEndian.Uint16(bs[6:8])

		var b byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		e.RunningStatus = b & 0x7

		d.Events = append(d.Events, e)
	}
	return
}
