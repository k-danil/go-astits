package psi

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v3/descriptor"
	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type SIT struct {
	TransmissionInfoDescriptors []descriptor.Descriptor `json:"_transmission_info_descriptors"`
	Services                    []SITService            `json:"_services"`
}

type SITService struct {
	Descriptors   []descriptor.Descriptor `json:"_descriptors"`
	ServiceID     uint16                  `json:"service_id"`
	RunningStatus RunningStatus           `json:"running_status"`
}

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

		// These 2 bytes also hold descriptors_loop_length; rewind for descriptor.Parse.
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
	n := 2 + descriptor.CalcLength(d.TransmissionInfoDescriptors)
	for _, s := range d.Services {
		n += 2 + 2 + descriptor.CalcLength(s.Descriptors)
	}
	return n
}

func (d *SIT) appendSection(dst []byte) []byte {
	dst = descriptor.AppendWithLength(dst, d.TransmissionInfoDescriptors)
	for _, s := range d.Services {
		loopLen := descriptor.CalcLength(s.Descriptors)
		dst = append(dst,
			byte(s.ServiceID>>8), byte(s.ServiceID),
			0x80|byte(s.RunningStatus)<<4|byte(loopLen>>8)&0xf,
			byte(loopLen))
		dst = descriptor.Append(dst, s.Descriptors)
	}
	return dst
}
