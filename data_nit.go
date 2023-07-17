package astits

import (
	"encoding/binary"
	"fmt"

	"github.com/asticode/go-astikit"
)

// NITData represents a NIT data
// Page: 29 | Chapter: 5.2.1 | Link: https://www.dvb.org/resources/public/standards/a38_dvb-si_specification.pdf
// (barbashov) the link above can be broken, alternative: https://dvb.org/wp-content/uploads/2019/12/a038_tm1217r37_en300468v1_17_1_-_rev-134_-_si_specification.pdf
type NITData struct {
	NetworkDescriptors []Descriptor
	TransportStreams   []NITDataTransportStream
	NetworkID          uint16
}

// NITDataTransportStream represents a NIT data transport stream
type NITDataTransportStream struct {
	TransportDescriptors []Descriptor
	TransportStreamID    uint16
	OriginalNetworkID    uint16
}

// parseNITSection parses a NIT section
func parseNITSection(i *astikit.BytesIterator, tableIDExtension uint16) (d *NITData, err error) {
	// Create data
	d = &NITData{NetworkID: tableIDExtension}

	// Network descriptors
	if d.NetworkDescriptors, err = parseDescriptors(i); err != nil {
		err = fmt.Errorf("astits: parsing descriptors failed: %w", err)
		return
	}

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
		ts := NITDataTransportStream{}

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
		if ts.TransportDescriptors, err = parseDescriptors(i); err != nil {
			err = fmt.Errorf("astits: parsing descriptors failed: %w", err)
			return
		}

		// Append transport stream
		d.TransportStreams = append(d.TransportStreams, ts)
	}
	return
}
