package psi

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/descriptor"
	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

// Running statuses
const (
	RunningStatusNotRunning          = 1
	RunningStatusPausing             = 3
	RunningStatusRunning             = 4
	RunningStatusServiceOffAir       = 5
	RunningStatusStartsInAFewSeconds = 2
	RunningStatusUndefined           = 0
)

// SDT represents an SDT data
// Page: 33 | Chapter: 5.2.3 | Link: https://www.dvb.org/resources/public/standards/a38_dvb-si_specification.pdf
// (barbashov) the link above can be broken, alternative: https://dvb.org/wp-content/uploads/2019/12/a038_tm1217r37_en300468v1_17_1_-_rev-134_-_si_specification.pdf
type SDT struct {
	Services          []SDTService
	OriginalNetworkID uint16
	TransportStreamID uint16
}

// SDTService represents an SDT data service
type SDTService struct {
	Descriptors            []descriptor.Descriptor
	ServiceID              uint16
	HasEITPresentFollowing bool // When true indicates that EIT present/following information for the service is present in the current TS
	HasEITSchedule         bool // When true indicates that EIT schedule information for the service is present in the current TS
	HasFreeCSAMode         bool // When true indicates that access to one or more streams may be controlled by a CA system.
	RunningStatus          uint8
}

// parseSDTSection parses an SDT section
func parseSDTSection(i *bytesiter.Iterator, offsetSectionsEnd int, tableIDExtension uint16) (d *SDT, err error) {
	d = &SDT{TransportStreamID: tableIDExtension}

	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d.OriginalNetworkID = binary.BigEndian.Uint16(bs)

	// Reserved for future use
	i.Skip(1)

	for i.Offset() < offsetSectionsEnd {
		s := SDTService{}

		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}

		s.ServiceID = binary.BigEndian.Uint16(bs)

		var b byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		s.HasEITSchedule = uint8(b&0x2) > 0

		s.HasEITPresentFollowing = uint8(b&0x1) > 0

		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		s.RunningStatus = b >> 5

		s.HasFreeCSAMode = uint8(b&0x10) > 0

		// We need to rewind since the current byte is used by the descriptor as well
		i.Skip(-1)

		var dn int
		if s.Descriptors, dn, err = descriptor.Parse(i.Bytes()); err != nil {
			err = fmt.Errorf("astits: parsing descriptors failed: %w", err)
			return
		}
		i.Skip(dn)

		d.Services = append(d.Services, s)
	}
	return
}

func (d *SDT) CalcSectionLength() int {
	n := 3 // original_network_id + reserved_future_use
	for _, s := range d.Services {
		n += 5 + descriptor.CalcLength(s.Descriptors) // service_id + 2 flag bytes + descriptors_loop_length_lo
	}
	return n
}

func (d *SDT) appendSection(dst []byte) []byte {
	dst = append(dst, byte(d.OriginalNetworkID>>8), byte(d.OriginalNetworkID), 0xff) // ONID + reserved_future_use
	for _, s := range d.Services {
		loopLen := descriptor.CalcLength(s.Descriptors)
		dst = append(dst,
			byte(s.ServiceID>>8), byte(s.ServiceID),
			0xfc|util.B2U(s.HasEITSchedule)<<1|util.B2U(s.HasEITPresentFollowing), // reserved(6) + EIT_schedule + EIT_present_following
			s.RunningStatus<<5|util.B2U(s.HasFreeCSAMode)<<4|byte(loopLen>>8)&0xf, // running_status(3) + free_CA(1) + descriptors_loop_length(12)
			byte(loopLen))
		dst = descriptor.Append(dst, s.Descriptors)
	}
	return dst
}
