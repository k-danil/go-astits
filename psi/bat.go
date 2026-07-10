package psi

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/descriptor"
	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// BAT represents a BAT: the bouquet association table groups services into a
// bouquet, carrying bouquet-level descriptors and the transport streams that
// make up the bouquet. Same shape as the NIT, keyed by bouquet instead of
// network.
// Page: 32 | Chapter: 5.2.2 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type BAT struct {
	BouquetDescriptors []descriptor.Descriptor
	TransportStreams   []BATTransportStream
	BouquetID          uint16
}

// BATTransportStream represents a transport stream of a BAT
type BATTransportStream struct {
	TransportDescriptors []descriptor.Descriptor
	TransportStreamID    uint16
	OriginalNetworkID    uint16
}

// parseBATSection parses a BAT section
func parseBATSection(i *bytesiter.Iterator, tableIDExtension uint16) (d *BAT, err error) {
	d = &BAT{BouquetID: tableIDExtension}

	var dn int
	if d.BouquetDescriptors, dn, err = descriptor.Parse(i.Bytes()); err != nil {
		err = fmt.Errorf("astits: parsing descriptors failed: %w", err)
		return
	}
	i.Skip(dn)

	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	transportStreamLoopLength := int(binary.BigEndian.Uint16(bs) & 0xfff)

	offsetEnd := i.Offset() + transportStreamLoopLength
	for i.Offset() < offsetEnd {
		s := BATTransportStream{}

		if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		val := binary.BigEndian.Uint32(bs)

		s.TransportStreamID = uint16(val >> 16)
		s.OriginalNetworkID = uint16(val)

		if s.TransportDescriptors, dn, err = descriptor.Parse(i.Bytes()); err != nil {
			err = fmt.Errorf("astits: parsing descriptors failed: %w", err)
			return
		}
		i.Skip(dn)

		d.TransportStreams = append(d.TransportStreams, s)
	}
	return
}
