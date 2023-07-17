package astits

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/asticode/go-astikit"
)

// EITData represents an EIT data
// Page: 36 | Chapter: 5.2.4 | Link: https://www.dvb.org/resources/public/standards/a38_dvb-si_specification.pdf
// (barbashov) the link above can be broken, alternative: https://dvb.org/wp-content/uploads/2019/12/a038_tm1217r37_en300468v1_17_1_-_rev-134_-_si_specification.pdf
type EITData struct {
	Events                   []EITDataEvent
	OriginalNetworkID        uint16
	ServiceID                uint16
	TransportStreamID        uint16
	LastTableID              uint8
	SegmentLastSectionNumber uint8
}

// EITDataEvent represents an EIT data event
type EITDataEvent struct {
	Descriptors    []Descriptor
	Duration       time.Duration
	StartTime      time.Time
	EventID        uint16
	HasFreeCSAMode bool // When true indicates that access to one or more streams may be controlled by a CA system.
	RunningStatus  uint8
}

// parseEITSection parses an EIT section
func parseEITSection(i *astikit.BytesIterator, offsetSectionsEnd int, tableIDExtension uint16) (d *EITData, err error) {
	// Create data
	d = &EITData{ServiceID: tableIDExtension}

	// Get next 2 bytes
	var bs []byte
	if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	val := binary.BigEndian.Uint32(bs)
	// Transport stream ID
	d.TransportStreamID = uint16(val >> 16)
	// Original network ID
	d.OriginalNetworkID = uint16(val)

	// Get next byte
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Segment last section number
	d.SegmentLastSectionNumber = b

	// Get next byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Last table ID
	d.LastTableID = b

	// Loop until end of section data is reached
	for i.Offset() < offsetSectionsEnd {
		// Get next 2 bytes
		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}

		// Event ID
		var e = EITDataEvent{}
		e.EventID = binary.BigEndian.Uint16(bs)

		// Start time
		if e.StartTime, err = parseDVBTime(i); err != nil {
			err = fmt.Errorf("astits: parsing DVB time")
			return
		}

		// Duration
		if e.Duration, err = parseDVBDurationSeconds(i); err != nil {
			err = fmt.Errorf("astits: parsing DVB duration seconds failed: %w", err)
			return
		}

		// Get next byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		// Running status
		e.RunningStatus = b >> 5

		// Free CA mode
		e.HasFreeCSAMode = b&0x10 > 0

		// We need to rewind since the current byte is used by the descriptor as well
		i.Skip(-1)

		// Descriptors
		if e.Descriptors, err = parseDescriptors(i); err != nil {
			err = fmt.Errorf("astits: parsing descriptors failed: %w", err)
			return
		}

		// Add event
		d.Events = append(d.Events, e)
	}
	return
}
