package psi

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/k-danil/go-astits/v2/descriptor"
	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/dvb"
)

// EIT represents an EIT data
// Page: 36 | Chapter: 5.2.4 | Link: https://www.dvb.org/resources/public/standards/a38_dvb-si_specification.pdf
// (barbashov) the link above can be broken, alternative: https://dvb.org/wp-content/uploads/2019/12/a038_tm1217r37_en300468v1_17_1_-_rev-134_-_si_specification.pdf
type EIT struct {
	Events                   []EITEvent
	OriginalNetworkID        uint16
	ServiceID                uint16
	TransportStreamID        uint16
	LastTableID              uint8
	SegmentLastSectionNumber uint8
}

// EITEvent represents an EIT data event
type EITEvent struct {
	Descriptors    []descriptor.Descriptor
	Duration       time.Duration
	StartTime      time.Time
	EventID        uint16
	HasFreeCSAMode bool // When true indicates that access to one or more streams may be controlled by a CA system.
	RunningStatus  uint8
}

// parseEITSection parses an EIT section
func parseEITSection(i *bytesiter.Iterator, offsetSectionsEnd int, tableIDExtension uint16) (d *EIT, err error) {
	d = &EIT{ServiceID: tableIDExtension}

	var bs []byte
	if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	val := binary.BigEndian.Uint32(bs)
	d.TransportStreamID = uint16(val >> 16)
	d.OriginalNetworkID = uint16(val)

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	d.SegmentLastSectionNumber = b

	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	d.LastTableID = b

	for i.Offset() < offsetSectionsEnd {
		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}

		var e = EITEvent{}
		e.EventID = binary.BigEndian.Uint16(bs)

		if e.StartTime, err = dvb.ParseTime(i); err != nil {
			err = fmt.Errorf("astits: parsing DVB time failed: %w", err)
			return
		}

		if e.Duration, err = dvb.ParseDurationSeconds(i); err != nil {
			err = fmt.Errorf("astits: parsing DVB duration seconds failed: %w", err)
			return
		}

		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		e.RunningStatus = b >> 5

		e.HasFreeCSAMode = b&0x10 > 0

		// We need to rewind since the current byte is used by the descriptor as well
		i.Skip(-1)

		var dn int
		if e.Descriptors, dn, err = descriptor.Parse(i.Bytes()); err != nil {
			err = fmt.Errorf("astits: parsing descriptors failed: %w", err)
			return
		}
		i.Skip(dn)

		d.Events = append(d.Events, e)
	}
	return
}
