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
	d = &NIT{NetworkID: tableIDExtension}

	var dn int
	if d.NetworkDescriptors, dn, err = descriptor.Parse(i.Bytes()); err != nil {
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
		ts := NITTransportStream{}

		if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		val := binary.BigEndian.Uint32(bs)

		ts.TransportStreamID = uint16(val >> 16)
		ts.OriginalNetworkID = uint16(val)

		if ts.TransportDescriptors, dn, err = descriptor.Parse(i.Bytes()); err != nil {
			err = fmt.Errorf("astits: parsing descriptors failed: %w", err)
			return
		}
		i.Skip(dn)

		d.TransportStreams = append(d.TransportStreams, ts)
	}
	return
}

func (d *NIT) transportStreamsLength() (n int) {
	for _, ts := range d.TransportStreams {
		n += 6 + descriptor.CalcLength(ts.TransportDescriptors) // TSID + ONID + transport_descriptors_length prefix
	}
	return
}

func (d *NIT) CalcSectionLength() int {
	// network_descriptors_length prefix + network descriptors + transport_stream_loop_length + TS loop
	return 2 + descriptor.CalcLength(d.NetworkDescriptors) + 2 + d.transportStreamsLength()
}

func (d *NIT) appendSection(dst []byte) []byte {
	dst = descriptor.AppendWithLength(dst, d.NetworkDescriptors)
	loopLen := d.transportStreamsLength()
	dst = append(dst, 0xf0|byte(loopLen>>8)&0xf, byte(loopLen)) // reserved(4) + transport_stream_loop_length(12)
	for _, ts := range d.TransportStreams {
		dst = append(dst,
			byte(ts.TransportStreamID>>8), byte(ts.TransportStreamID),
			byte(ts.OriginalNetworkID>>8), byte(ts.OriginalNetworkID))
		dst = descriptor.AppendWithLength(dst, ts.TransportDescriptors)
	}
	return dst
}
