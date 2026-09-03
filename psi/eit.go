package psi

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/k-danil/go-astits/v3/descriptor"
	"github.com/k-danil/go-astits/v3/internal/bytesiter"
	"github.com/k-danil/go-astits/v3/internal/dvb"
	"github.com/k-danil/go-astits/v3/internal/util"
)

type EIT struct {
	Events                   []EITEvent `json:"_events"`
	OriginalNetworkID        uint16     `json:"original_network_id"`
	ServiceID                uint16     `json:"service_id"`
	TransportStreamID        uint16     `json:"transport_stream_id"`
	LastTableID              TableID    `json:"last_table_id"`
	SegmentLastSectionNumber uint8      `json:"segment_last_section_number"`
}

type EITEvent struct {
	Descriptors    []descriptor.Descriptor `json:"_descriptors"`
	Duration       time.Duration           `json:"duration"`
	StartTime      time.Time               `json:"start_time"`
	EventID        uint16                  `json:"event_id"`
	HasFreeCSAMode bool                    `json:"free_CA_mode"`
	RunningStatus  RunningStatus           `json:"running_status"`
}

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

	d.LastTableID = TableID(b)

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

		e.RunningStatus = RunningStatus(b >> 5)

		e.HasFreeCSAMode = b&0x10 > 0

		// The low bits of this byte are descriptors_loop_length; rewind for descriptor.Parse.
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

func (d *EIT) CalcSectionLength() int {
	n := 2 + 2 + 1 + 1
	for _, e := range d.Events {
		n += 2 + 5 + 3 + 2 + descriptor.CalcLength(e.Descriptors)
	}
	return n
}

func (d *EIT) appendSection(dst []byte) []byte {
	dst = append(dst,
		byte(d.TransportStreamID>>8), byte(d.TransportStreamID),
		byte(d.OriginalNetworkID>>8), byte(d.OriginalNetworkID),
		d.SegmentLastSectionNumber, byte(d.LastTableID))
	for _, e := range d.Events {
		loopLen := descriptor.CalcLength(e.Descriptors)
		dst = append(dst, byte(e.EventID>>8), byte(e.EventID))
		dst = dvb.AppendTime(dst, e.StartTime)
		dst = dvb.AppendDurationSeconds(dst, e.Duration)
		dst = append(dst,
			byte(e.RunningStatus)<<5|util.B2U(e.HasFreeCSAMode)<<4|byte(loopLen>>8)&0xf,
			byte(loopLen))
		dst = descriptor.Append(dst, e.Descriptors)
	}
	return dst
}
