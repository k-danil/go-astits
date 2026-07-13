package psi

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/descriptor"
	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// SIT represents a SIT: the selection information table describes the services
// carried in a partial (recorded) transport stream — transmission-wide
// descriptors followed by a per-service loop with running status.
// Page: 40 | Chapter: 5.2.10 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type SIT struct {
	TransmissionInfoDescriptors []descriptor.Descriptor `json:"_transmission_info_descriptors"`
	Services                    []SITService            `json:"_services"`
}

// SITService represents one service entry of a SIT
type SITService struct {
	Descriptors   []descriptor.Descriptor `json:"_descriptors"`
	ServiceID     uint16                  `json:"service_id"`
	RunningStatus RunningStatus           `json:"running_status"`
}

// parseSITSection parses a SIT section
func parseSITSection(i *bytesiter.Iterator, offsetSectionsEnd int) (d *SIT, err error) {
	d = &SIT{}

	var dn int
	if d.TransmissionInfoDescriptors, dn, err = descriptor.Parse(i.Bytes()); err != nil {
		err = fmt.Errorf("astits: parsing descriptors failed: %w", err)
		return
	}
	i.Skip(dn)

	var bs []byte
	for i.Offset() < offsetSectionsEnd {
		s := SITService{}

		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		s.ServiceID = binary.BigEndian.Uint16(bs)

		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		s.RunningStatus = RunningStatus(binary.BigEndian.Uint16(bs) >> 12 & 0x7)

		// The 2 bytes just read pack running_status over the descriptor-loop
		// length; rewind so descriptor.Parse consumes them as its prefix.
		i.Skip(-2)
		if s.Descriptors, dn, err = descriptor.Parse(i.Bytes()); err != nil {
			err = fmt.Errorf("astits: parsing descriptors failed: %w", err)
			return
		}
		i.Skip(dn)

		d.Services = append(d.Services, s)
	}
	return
}

func (d *SIT) CalcSectionLength() int {
	n := 2 + descriptor.CalcLength(d.TransmissionInfoDescriptors) // transmission_info_loop_length prefix + descriptors
	for _, s := range d.Services {
		n += 4 + descriptor.CalcLength(s.Descriptors) // service_id + 2 status/length bytes + descriptors
	}
	return n
}

func (d *SIT) appendSection(dst []byte) []byte {
	dst = descriptor.AppendWithLength(dst, d.TransmissionInfoDescriptors)
	for _, s := range d.Services {
		loopLen := descriptor.CalcLength(s.Descriptors)
		dst = append(dst,
			byte(s.ServiceID>>8), byte(s.ServiceID),
			0x80|byte(s.RunningStatus)<<4|byte(loopLen>>8)&0xf, // reserved(1) + running_status(3) + service_loop_length(12)
			byte(loopLen))
		dst = descriptor.Append(dst, s.Descriptors)
	}
	return dst
}
