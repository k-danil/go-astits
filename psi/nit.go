package psi

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/descriptor"
	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// NIT represents a NIT data
// Page: 29 | Chapter: 5.2.1 | Link: https://www.dvb.org/resources/public/standards/a38_dvb-si_specification.pdf
// (barbashov) the link above can be broken, alternative: https://dvb.org/wp-content/uploads/2019/12/a038_tm1217r37_en300468v1_17_1_-_rev-134_-_si_specification.pdf
type NIT struct {
	NetworkDescriptors []descriptor.Descriptor
	TransportStreams   []NITTransportStream
	NetworkID          uint16
}

// NITTransportStream represents a NIT data transport stream
type NITTransportStream struct {
	TransportDescriptors []descriptor.Descriptor
	TransportStreamID    uint16
	OriginalNetworkID    uint16
}

// parseNITSection parses a NIT section
func parseNITSection(i *bytesiter.Iterator, tableIDExtension uint16) (d *NIT, err error) {
	// Create data
	d = &NIT{NetworkID: tableIDExtension}

	// Network descriptors
	var dn int
	if d.NetworkDescriptors, dn, err = descriptor.Parse(i.Bytes()); err != nil {
		err = fmt.Errorf("astits: parsing descriptors failed: %w", err)
		return
	}
	i.Skip(dn)

	// Get next bytes
	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	// Transport stream loop length
	transportStreamLoopLength := int(binary.BigEndian.Uint16(bs) & 0xfff)

	// Transport stream loop
	offsetEnd := i.Offset() + transportStreamLoopLength
	for i.Offset() < offsetEnd {
		// Create transport stream
		ts := NITTransportStream{}

		// Get next bytes
		if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		val := binary.BigEndian.Uint32(bs)

		// Transport stream ID
		ts.TransportStreamID = uint16(val >> 16)
		// Original network ID
		ts.OriginalNetworkID = uint16(val)

		// Transport descriptors
		if ts.TransportDescriptors, dn, err = descriptor.Parse(i.Bytes()); err != nil {
			err = fmt.Errorf("astits: parsing descriptors failed: %w", err)
			return
		}
		i.Skip(dn)

		// Append transport stream
		d.TransportStreams = append(d.TransportStreams, ts)
	}
	return
}
