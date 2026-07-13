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
	BouquetDescriptors []descriptor.Descriptor `json:"_bouquet_descriptors"`
	TransportStreams   []BATTransportStream    `json:"_transport_streams"`
	BouquetID          uint16                  `json:"bouquet_id"`
}

// BATTransportStream represents a transport stream of a BAT
type BATTransportStream struct {
	TransportDescriptors []descriptor.Descriptor `json:"_transport_descriptors"`
	TransportStreamID    uint16                  `json:"transport_stream_id"`
	OriginalNetworkID    uint16                  `json:"original_network_id"`
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

func (d *BAT) transportStreamsLength() (n int) {
	for _, ts := range d.TransportStreams {
		n += 6 + descriptor.CalcLength(ts.TransportDescriptors) // TSID + ONID + transport_descriptors_length prefix
	}
	return
}

func (d *BAT) CalcSectionLength() int {
	// bouquet_descriptors_length prefix + bouquet descriptors + transport_stream_loop_length + TS loop
	return 2 + descriptor.CalcLength(d.BouquetDescriptors) + 2 + d.transportStreamsLength()
}

func (d *BAT) appendSection(dst []byte) []byte {
	dst = descriptor.AppendWithLength(dst, d.BouquetDescriptors)
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
