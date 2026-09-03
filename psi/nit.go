package psi

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v3/descriptor"
	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type NIT struct {
	NetworkDescriptors []descriptor.Descriptor `json:"_network_descriptors"`
	TransportStreams   []NITTransportStream    `json:"_transport_streams"`
	NetworkID          uint16                  `json:"network_id"`
}

type NITTransportStream struct {
	TransportDescriptors []descriptor.Descriptor `json:"_transport_descriptors"`
	TransportStreamID    uint16                  `json:"transport_stream_id"`
	OriginalNetworkID    uint16                  `json:"original_network_id"`
}

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
		n += 2 + 2 + 2 + descriptor.CalcLength(ts.TransportDescriptors)
	}
	return
}

func (d *NIT) CalcSectionLength() int {
	return 2 + descriptor.CalcLength(d.NetworkDescriptors) + 2 + d.transportStreamsLength()
}

func (d *NIT) appendSection(dst []byte) []byte {
	dst = descriptor.AppendWithLength(dst, d.NetworkDescriptors)
	loopLen := d.transportStreamsLength()
	dst = append(dst, 0xf0|byte(loopLen>>8)&0xf, byte(loopLen))
	for _, ts := range d.TransportStreams {
		dst = append(dst,
			byte(ts.TransportStreamID>>8), byte(ts.TransportStreamID),
			byte(ts.OriginalNetworkID>>8), byte(ts.OriginalNetworkID))
		dst = descriptor.AppendWithLength(dst, ts.TransportDescriptors)
	}
	return dst
}
