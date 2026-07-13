package psi

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// rstEventBytesSize is one running-status entry: 4×16-bit IDs + a byte holding
// reserved_future_use (5 bits) and running_status (3 bits).
const rstEventBytesSize = 9

// RST represents an RST: the running status table signals rapid updates to the
// running status of events. It has neither a syntax header nor a CRC.
// Page: 38 | Chapter: 5.2.6 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type RST struct {
	Events []RSTEvent `json:"_events"`
}

// RSTEvent represents one running-status entry of an RST
type RSTEvent struct {
	TransportStreamID uint16        `json:"transport_stream_id"`
	OriginalNetworkID uint16        `json:"original_network_id"`
	ServiceID         uint16        `json:"service_id"`
	EventID           uint16        `json:"event_id"`
	RunningStatus     RunningStatus `json:"running_status"`
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
		e.RunningStatus = RunningStatus(b & 0x7)

		d.Events = append(d.Events, e)
	}
	return
}

func (d *RST) CalcSectionLength() int { return rstEventBytesSize * len(d.Events) }

func (d *RST) appendSection(dst []byte) []byte {
	for _, e := range d.Events {
		dst = append(dst,
			byte(e.TransportStreamID>>8), byte(e.TransportStreamID),
			byte(e.OriginalNetworkID>>8), byte(e.OriginalNetworkID),
			byte(e.ServiceID>>8), byte(e.ServiceID),
			byte(e.EventID>>8), byte(e.EventID),
			0xf8|byte(e.RunningStatus)&0x7) // reserved_future_use (5 bits, 1) + running_status
	}
	return dst
}
