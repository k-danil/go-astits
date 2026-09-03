package psi

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v3/descriptor"
	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type BAT struct {
	BouquetDescriptors []descriptor.Descriptor `json:"_bouquet_descriptors"`
	TransportStreams   []BATTransportStream    `json:"_transport_streams"`
	BouquetID          uint16                  `json:"bouquet_id"`
}

type BATTransportStream struct {
	TransportDescriptors []descriptor.Descriptor `json:"_transport_descriptors"`
	TransportStreamID    uint16                  `json:"transport_stream_id"`
	OriginalNetworkID    uint16                  `json:"original_network_id"`
}

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
		n += 2 + 2 + 2 + descriptor.CalcLength(ts.TransportDescriptors)
	}
	return
}

func (d *BAT) CalcSectionLength() int {
	return 2 + descriptor.CalcLength(d.BouquetDescriptors) + 2 + d.transportStreamsLength()
}

func (d *BAT) appendSection(dst []byte) []byte {
	dst = descriptor.AppendWithLength(dst, d.BouquetDescriptors)
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
