package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

const (
	metadataApplicationFormatEscape = 0xffff
	metadataFormatEscape            = 0xff
)

const (
	metadataDecoderConfigInDescriptor   = 0x1
	metadataDecoderConfigIdentification = 0x3
	metadataDecoderConfigServiceRef     = 0x4
	metadataDecoderConfigReservedLow    = 0x5
	metadataDecoderConfigReservedHigh   = 0x6
)

// Metadata is the MPEG-2 systems metadata_descriptor (ISO/IEC 13818-1).
type Metadata struct {
	ServiceIdentificationRecord         []byte
	DecoderConfig                       []byte
	DecConfigIdentificationRecord       []byte
	ReservedData                        []byte
	PrivateData                         []byte
	MetadataApplicationFormatIdentifier uint32
	MetadataFormatIdentifier            uint32
	Header                              Header
	MetadataApplicationFormat           uint16
	MetadataFormat                      uint8
	MetadataServiceID                   uint8
	DecoderConfigFlags                  uint8
	DecoderConfigMetadataServiceID      uint8
	DSMCCFlag                           bool
}

func nextMetadataRecord(i *bytesiter.Iterator) (bs []byte, err error) {
	var n byte
	if n, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	if bs, err = i.NextBytes(int(n)); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func newDescriptorMetadata(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &Metadata{
		Header:                    h,
		MetadataApplicationFormat: binary.BigEndian.Uint16(bs),
	}
	dd = d

	if d.MetadataApplicationFormat == metadataApplicationFormatEscape {
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

	if d.MetadataFormat == metadataFormatEscape {
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
	d.DecoderConfigFlags = b >> 5 & 0x7
	d.DSMCCFlag = b>>4&0x1 > 0

	if d.DSMCCFlag {
		if d.ServiceIdentificationRecord, err = nextMetadataRecord(i); err != nil {
			return
		}
	}

	switch d.DecoderConfigFlags {
	case metadataDecoderConfigInDescriptor:
		if d.DecoderConfig, err = nextMetadataRecord(i); err != nil {
			return
		}
	case metadataDecoderConfigIdentification:
		if d.DecConfigIdentificationRecord, err = nextMetadataRecord(i); err != nil {
			return
		}
	case metadataDecoderConfigServiceRef:
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.DecoderConfigMetadataServiceID = b
	case metadataDecoderConfigReservedLow, metadataDecoderConfigReservedHigh:
		if d.ReservedData, err = nextMetadataRecord(i); err != nil {
			return
		}
	}

	if i.Offset() < offsetEnd {
		if d.PrivateData, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *Metadata) CalcLength() int {
	ret := 2
	if d.MetadataApplicationFormat == metadataApplicationFormatEscape {
		ret += 4
	}
	ret++
	if d.MetadataFormat == metadataFormatEscape {
		ret += 4
	}
	ret += 2

	if d.DSMCCFlag {
		ret += 1 + len(d.ServiceIdentificationRecord)
	}

	switch d.DecoderConfigFlags {
	case metadataDecoderConfigInDescriptor:
		ret += 1 + len(d.DecoderConfig)
	case metadataDecoderConfigIdentification:
		ret += 1 + len(d.DecConfigIdentificationRecord)
	case metadataDecoderConfigServiceRef:
		ret++
	case metadataDecoderConfigReservedLow, metadataDecoderConfigReservedHigh:
		ret += 1 + len(d.ReservedData)
	}

	return ret + len(d.PrivateData)
}

func appendMetadataRecord(dst, record []byte) []byte {
	dst = append(dst, uint8(len(record)))
	return append(dst, record...)
}

func (d *Metadata) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, byte(d.MetadataApplicationFormat>>8), byte(d.MetadataApplicationFormat))

	if d.MetadataApplicationFormat == metadataApplicationFormatEscape {
		dst = append(dst, byte(d.MetadataApplicationFormatIdentifier>>24), byte(d.MetadataApplicationFormatIdentifier>>16), byte(d.MetadataApplicationFormatIdentifier>>8), byte(d.MetadataApplicationFormatIdentifier))
	}

	dst = append(dst, d.MetadataFormat)
	if d.MetadataFormat == metadataFormatEscape {
		dst = append(dst, byte(d.MetadataFormatIdentifier>>24), byte(d.MetadataFormatIdentifier>>16), byte(d.MetadataFormatIdentifier>>8), byte(d.MetadataFormatIdentifier))
	}

	dst = append(dst, d.MetadataServiceID)
	dst = append(dst, d.DecoderConfigFlags&0x7<<5|util.B2U(d.DSMCCFlag)<<4|0x0f)

	if d.DSMCCFlag {
		dst = appendMetadataRecord(dst, d.ServiceIdentificationRecord)
	}

	switch d.DecoderConfigFlags {
	case metadataDecoderConfigInDescriptor:
		dst = appendMetadataRecord(dst, d.DecoderConfig)
	case metadataDecoderConfigIdentification:
		dst = appendMetadataRecord(dst, d.DecConfigIdentificationRecord)
	case metadataDecoderConfigServiceRef:
		dst = append(dst, d.DecoderConfigMetadataServiceID)
	case metadataDecoderConfigReservedLow, metadataDecoderConfigReservedHigh:
		dst = appendMetadataRecord(dst, d.ReservedData)
	}

	return append(dst, d.PrivateData...)
}
