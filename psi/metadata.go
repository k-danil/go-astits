package psi

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

// Metadata represents a metadata_section (ISO/IEC 13818-1 §2.12.4): a fragment
// of a metadata service. Its header deviates from the standard PSI long form:
// random_access_indicator and decoder_config_flag occupy the section header's
// reserved bits (see SectionHeader), and metadata_service_id /
// section_fragment_indication replace table_id_extension / the syntax header's
// reserved bits. The metadata Access Unit payload is carried verbatim.
type Metadata struct {
	MetadataBytes             []byte `json:"metadata_byte"`
	MetadataServiceID         uint8  `json:"metadata_service_id"`
	SectionFragmentIndication uint8  `json:"section_fragment_indication"`
	VersionNumber             uint8  `json:"version_number"`
	SectionNumber             uint8  `json:"section_number"`
	LastSectionNumber         uint8  `json:"last_section_number"`
	CurrentNextIndicator      bool   `json:"current_next_indicator"`
}

func parseMetadataSection(i *bytesiter.Iterator, offsetSectionsEnd int) (d *Metadata, err error) {
	d = &Metadata{}

	if d.MetadataServiceID, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	i.Skip(1) // reserved

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.SectionFragmentIndication = b >> 6 & 0x3
	d.VersionNumber = b >> 1 & 0x1f
	d.CurrentNextIndicator = b&0x1 > 0

	if d.SectionNumber, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	if d.LastSectionNumber, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	length := offsetSectionsEnd - i.Offset()
	if length > 0 {
		if d.MetadataBytes, err = i.NextBytes(length); err != nil {
			err = fmt.Errorf("astits: fetching metadata bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *Metadata) CalcSectionLength() int {
	return 5 + len(d.MetadataBytes) // service_id + reserved + fragment/version/current + section_number + last_section_number
}

func (d *Metadata) appendSection(dst []byte) []byte {
	dst = append(dst,
		d.MetadataServiceID,
		0xff,
		d.SectionFragmentIndication&0x3<<6|d.VersionNumber&0x1f<<1|util.B2U(d.CurrentNextIndicator),
		d.SectionNumber,
		d.LastSectionNumber)
	return append(dst, d.MetadataBytes...)
}
