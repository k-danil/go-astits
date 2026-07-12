package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

// MetadataPointer is the MPEG-2 systems metadata_pointer_descriptor (ISO/IEC 13818-1).
type MetadataPointer struct {
	MetadataLocatorRecord               []byte
	PrivateData                         []byte
	MetadataApplicationFormatIdentifier uint32
	MetadataFormatIdentifier            uint32
	Header                              Header
	MetadataApplicationFormat           uint16
	ProgramNumber                       uint16
	TransportStreamLocation             uint16
	TransportStreamID                   uint16
	MPEGCarriageFlags                   uint8
	MetadataFormat                      uint8
	MetadataServiceID                   uint8
	MetadataLocatorRecordFlag           bool
}

const (
	metadataApplicationFormatIdentifierGate = 0xffff
	metadataFormatIdentifierGate            = 0xff
	carriageFlagProgramStream               = 2
	carriageFlagDifferentTransportStream    = 1
)

func newDescriptorMetadataPointer(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &MetadataPointer{
		Header:                    h,
		MetadataApplicationFormat: binary.BigEndian.Uint16(bs),
	}
	dd = d

	if d.MetadataApplicationFormat == metadataApplicationFormatIdentifierGate {
		if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		d.MetadataApplicationFormatIdentifier = binary.BigEndian.Uint32(bs)
	}

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.MetadataFormat = b

	if d.MetadataFormat == metadataFormatIdentifierGate {
		if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		d.MetadataFormatIdentifier = binary.BigEndian.Uint32(bs)
	}

	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.MetadataServiceID = b

	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.MetadataLocatorRecordFlag = b&0x80 > 0
	d.MPEGCarriageFlags = b >> 5 & 0x3

	if d.MetadataLocatorRecordFlag {
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		if d.MetadataLocatorRecord, err = i.NextBytes(int(b)); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}

	if d.MPEGCarriageFlags <= carriageFlagProgramStream {
		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		d.ProgramNumber = binary.BigEndian.Uint16(bs)
	}

	if d.MPEGCarriageFlags == carriageFlagDifferentTransportStream {
		if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		d.TransportStreamLocation = binary.BigEndian.Uint16(bs)
		d.TransportStreamID = binary.BigEndian.Uint16(bs[2:])
	}

	if i.Offset() < offsetEnd {
		if d.PrivateData, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *MetadataPointer) CalcLength() int {
	ret := 2
	if d.MetadataApplicationFormat == metadataApplicationFormatIdentifierGate {
		ret += 4
	}
	ret++
	if d.MetadataFormat == metadataFormatIdentifierGate {
		ret += 4
	}
	ret += 2
	if d.MetadataLocatorRecordFlag {
		ret += 1 + len(d.MetadataLocatorRecord)
	}
	if d.MPEGCarriageFlags <= carriageFlagProgramStream {
		ret += 2
	}
	if d.MPEGCarriageFlags == carriageFlagDifferentTransportStream {
		ret += 4
	}
	ret += len(d.PrivateData)
	return ret
}

func (d *MetadataPointer) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = binary.BigEndian.AppendUint16(dst, d.MetadataApplicationFormat)
	if d.MetadataApplicationFormat == metadataApplicationFormatIdentifierGate {
		dst = binary.BigEndian.AppendUint32(dst, d.MetadataApplicationFormatIdentifier)
	}
	dst = append(dst, d.MetadataFormat)
	if d.MetadataFormat == metadataFormatIdentifierGate {
		dst = binary.BigEndian.AppendUint32(dst, d.MetadataFormatIdentifier)
	}
	dst = append(dst, d.MetadataServiceID)
	dst = append(dst, util.B2U(d.MetadataLocatorRecordFlag)<<7|d.MPEGCarriageFlags&0x3<<5|0x1f)
	if d.MetadataLocatorRecordFlag {
		dst = append(dst, uint8(len(d.MetadataLocatorRecord)))
		dst = append(dst, d.MetadataLocatorRecord...)
	}
	if d.MPEGCarriageFlags <= carriageFlagProgramStream {
		dst = binary.BigEndian.AppendUint16(dst, d.ProgramNumber)
	}
	if d.MPEGCarriageFlags == carriageFlagDifferentTransportStream {
		dst = binary.BigEndian.AppendUint16(dst, d.TransportStreamLocation)
		dst = binary.BigEndian.AppendUint16(dst, d.TransportStreamID)
	}
	return append(dst, d.PrivateData...)
}
