package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
	"github.com/k-danil/go-astits/v3/internal/util"
)

type MetadataPointer struct {
	MetadataLocatorRecord               []byte `json:"metadata_locator_record"`
	PrivateData                         []byte `json:"private_data"`
	MetadataApplicationFormatIdentifier uint32 `json:"metadata_application_format_identifier"`
	MetadataFormatIdentifier            uint32 `json:"metadata_format_identifier"`
	Header                              Header `json:"_header"`
	MetadataApplicationFormat           uint16 `json:"metadata_application_format"`
	ProgramNumber                       uint16 `json:"program_number"`
	TransportStreamLocation             uint16 `json:"transport_stream_location"`
	TransportStreamID                   uint16 `json:"transport_stream_id"`
	MPEGCarriageFlags                   uint8  `json:"MPEG_carriage_flags"`
	MetadataFormat                      uint8  `json:"metadata_format"`
	MetadataServiceID                   uint8  `json:"metadata_service_id"`
	MetadataLocatorRecordFlag           bool   `json:"metadata_locator_record_flag"`
}

const (
	metadataApplicationFormatIdentifierGate = 0xffff
	metadataFormatIdentifierGate            = 0xff
	carriageFlagProgramStream               = 2
	carriageFlagDifferentTransportStream    = 1
	mpegCarriageFlagsMask                   = 0x3
)

// Sizing and writing must branch on the same truncated value or they disagree.
func (d *MetadataPointer) mpegCarriageFlags() uint8 {
	return d.MPEGCarriageFlags & mpegCarriageFlagsMask
}

func newDescriptorMetadataPointer(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &MetadataPointer{
		Header:                    h,
		MetadataApplicationFormat: binary.BigEndian.Uint16(bs),
	}
	dd = d

	if d.MetadataApplicationFormat == metadataApplicationFormatIdentifierGate {
		if bs, err = i.NextBytesNoCopy(4); err != nil {
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
		if bs, err = i.NextBytesNoCopy(4); err != nil {
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
		if bs, err = i.NextBytesNoCopy(2); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		d.ProgramNumber = binary.BigEndian.Uint16(bs)
	}

	if d.MPEGCarriageFlags == carriageFlagDifferentTransportStream {
		if bs, err = i.NextBytesNoCopy(4); err != nil {
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
	if d.mpegCarriageFlags() <= carriageFlagProgramStream {
		ret += 2
	}
	if d.mpegCarriageFlags() == carriageFlagDifferentTransportStream {
		ret += 4
	}
	ret += len(d.PrivateData)
	return ret
}

func (d *MetadataPointer) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	dst = binary.BigEndian.AppendUint16(dst, d.MetadataApplicationFormat)
	if d.MetadataApplicationFormat == metadataApplicationFormatIdentifierGate {
		dst = binary.BigEndian.AppendUint32(dst, d.MetadataApplicationFormatIdentifier)
	}
	dst = append(dst, d.MetadataFormat)
	if d.MetadataFormat == metadataFormatIdentifierGate {
		dst = binary.BigEndian.AppendUint32(dst, d.MetadataFormatIdentifier)
	}
	dst = append(dst, d.MetadataServiceID)
	dst = append(dst, util.B2U(d.MetadataLocatorRecordFlag)<<7|d.mpegCarriageFlags()<<5|0x1f)
	if d.MetadataLocatorRecordFlag {
		dst = append(dst, uint8(len(d.MetadataLocatorRecord)))
		dst = append(dst, d.MetadataLocatorRecord...)
	}
	if d.mpegCarriageFlags() <= carriageFlagProgramStream {
		dst = binary.BigEndian.AppendUint16(dst, d.ProgramNumber)
	}
	if d.mpegCarriageFlags() == carriageFlagDifferentTransportStream {
		dst = binary.BigEndian.AppendUint16(dst, d.TransportStreamLocation)
		dst = binary.BigEndian.AppendUint16(dst, d.TransportStreamID)
	}
	return append(dst, d.PrivateData...)
}
