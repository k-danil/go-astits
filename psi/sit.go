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
	TransmissionInfoDescriptors []descriptor.Descriptor
	Services                    []SITService
}

// SITService represents one service entry of a SIT
type SITService struct {
	Descriptors   []descriptor.Descriptor
	ServiceID     uint16
	RunningStatus uint8
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
		s.RunningStatus = uint8(binary.BigEndian.Uint16(bs) >> 12 & 0x7)

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
