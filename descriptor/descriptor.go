package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// Tag identifies a descriptor type on the wire.
type Tag uint8

// Descriptor tags
// Chapter: 6.1 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
const (
	TagAC3                        Tag = 0x6a
	TagAVCVideo                   Tag = 0x28
	TagComponent                  Tag = 0x50
	TagContent                    Tag = 0x54
	TagDataStreamAlignment        Tag = 0x6
	TagEnhancedAC3                Tag = 0x7a
	TagExtendedEvent              Tag = 0x4e
	TagExtension                  Tag = 0x7f
	TagISO639LanguageAndAudioType Tag = 0xa
	TagLocalTimeOffset            Tag = 0x58
	TagMaximumBitrate             Tag = 0xe
	TagNetworkName                Tag = 0x40
	TagParentalRating             Tag = 0x55
	TagPrivateDataIndicator       Tag = 0xf
	TagPrivateDataSpecifier       Tag = 0x5f
	TagRegistration               Tag = 0x5
	TagService                    Tag = 0x48
	TagShortEvent                 Tag = 0x4d
	TagStreamIdentifier           Tag = 0x52
	TagSubtitling                 Tag = 0x59
	TagTeletext                   Tag = 0x56
	TagVBIData                    Tag = 0x45
	TagVBITeletext                Tag = 0x46
)

// Parse parses a length-prefixed descriptor list; n is the number of bytes
// consumed (2-byte length prefix plus the descriptors).
func Parse(bs []byte) (ds []Descriptor, n int, err error) {
	i := bytesiter.New(bs)
	if ds, err = parseDescriptors(i); err != nil {
		return
	}
	return ds, i.Offset(), nil
}

func parseDescriptors(i *bytesiter.Iterator) (o []Descriptor, err error) {
	// Get next 2 bytes
	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	// Get length
	length := int(binary.BigEndian.Uint16(bs) & 0xfff)

	// Loop
	if length > 0 {
		curOffset := i.Offset()
		offsetEnd := i.Offset() + length

		var descrCount int
		for i.Offset() < offsetEnd {
			i.Skip(1)
			var b byte
			if b, err = i.NextByte(); err != nil {
				err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
				return
			}
			i.Skip(int(b))
			descrCount++
		}

		i.Seek(curOffset)

		o = make([]Descriptor, descrCount)

		for idx := range o {
			// Get next 2 bytes
			if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
				err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
				return
			}

			h := Header{
				Tag:    Tag(bs[0]),
				Length: bs[1],
			}

			// Parse data
			if h.Length > 0 {
				// Unfortunately there's no way to be sure the real descriptor length is the same as the one indicated
				// previously therefore we must fetch bytes in descriptor functions and seek at the end
				offsetDescriptorEnd := i.Offset() + int(h.Length)
				if o[idx], err = h.parseDescriptor(i, offsetDescriptorEnd); err != nil {
					err = fmt.Errorf("astits: parsing descriptor %x failed: %w", h.Tag, err)
					return
				}
				// Seek in iterator to make sure we move to the end of the descriptor since its content may be
				// corrupted
				i.Seek(offsetDescriptorEnd)
			} else if h.Tag >= userDefinedTagsStart && h.Tag != 0xff {
				// A zero-length descriptor is valid wire: represent it instead of
				// leaving a nil entry in the returned slice
				o[idx] = &UserDefined{Header: h}
			} else {
				o[idx] = &Unknown{Header: h}
			}
		}
	}
	return
}

// AppendWithLength appends the 2-byte descriptors length prefix followed by
// the serialized descriptors.
func AppendWithLength(dst []byte, ds []Descriptor) []byte {
	length := uint16(CalcLength(ds))
	dst = append(dst, byte(length>>8)|0xf0, byte(length))
	for _, d := range ds {
		dst = d.Append(dst)
	}
	return dst
}

// CalcLength returns the total serialized size of a descriptor list,
// including the 2-byte tag+length prefix of each entry.
func CalcLength(ds []Descriptor) (length int) {
	for _, d := range ds {
		length += 2 // tag and length
		length += d.CalcLength()
	}
	return
}

// Descriptor is a parsed DVB or MPEG descriptor. Concrete types carry the
// parsed fields; all serialize through CalcLength and Append.
type Descriptor interface {
	// CalcLength returns the value of the descriptor_length field: the body
	// size without the 2-byte tag+length prefix.
	CalcLength() int
	// Append appends the serialized descriptor (tag, length, body) to dst.
	Append(dst []byte) []byte
}

// Header is the 2-byte tag+length prefix common to every descriptor.
type Header struct {
	Tag    Tag // the tag defines the structure of the contained data following the descriptor length.
	Length uint8
}

// userDefinedTagsStart is the bottom of the user-defined tag range
// (0x80-0xfe); 0xff is forbidden by the spec.
const userDefinedTagsStart = 0x80

// parseDescriptor dispatches via a switch on purpose: an indirect call through
// a parser LUT defeats escape analysis and forces the iterator to the heap.
func (dh Header) parseDescriptor(i *bytesiter.Iterator, offsetEnd int) (d Descriptor, err error) {
	switch dh.Tag {
	case TagAC3:
		return newDescriptorAC3(i, dh, offsetEnd)
	case TagAVCVideo:
		return newDescriptorAVCVideo(i, dh, offsetEnd)
	case TagComponent:
		return newDescriptorComponent(i, dh, offsetEnd)
	case TagContent:
		return newDescriptorContent(i, dh, offsetEnd)
	case TagDataStreamAlignment:
		return newDescriptorDataStreamAlignment(i, dh, offsetEnd)
	case TagEnhancedAC3:
		return newDescriptorEnhancedAC3(i, dh, offsetEnd)
	case TagExtendedEvent:
		return newDescriptorExtendedEvent(i, dh, offsetEnd)
	case TagExtension:
		return newDescriptorExtension(i, dh, offsetEnd)
	case TagISO639LanguageAndAudioType:
		return newDescriptorISO639LanguageAndAudioType(i, dh, offsetEnd)
	case TagLocalTimeOffset:
		return newDescriptorLocalTimeOffset(i, dh, offsetEnd)
	case TagMaximumBitrate:
		return newDescriptorMaximumBitrate(i, dh, offsetEnd)
	case TagNetworkName:
		return newDescriptorNetworkName(i, dh, offsetEnd)
	case TagParentalRating:
		return newDescriptorParentalRating(i, dh, offsetEnd)
	case TagPrivateDataIndicator:
		return newDescriptorPrivateDataIndicator(i, dh, offsetEnd)
	case TagPrivateDataSpecifier:
		return newDescriptorPrivateDataSpecifier(i, dh, offsetEnd)
	case TagRegistration:
		return newDescriptorRegistration(i, dh, offsetEnd)
	case TagService:
		return newDescriptorService(i, dh, offsetEnd)
	case TagShortEvent:
		return newDescriptorShortEvent(i, dh, offsetEnd)
	case TagStreamIdentifier:
		return newDescriptorStreamIdentifier(i, dh, offsetEnd)
	case TagSubtitling:
		return newDescriptorSubtitling(i, dh, offsetEnd)
	case TagTeletext, TagVBITeletext:
		return newDescriptorTeletext(i, dh, offsetEnd)
	case TagVBIData:
		return newDescriptorVBIData(i, dh, offsetEnd)
	default:
		if dh.Tag >= userDefinedTagsStart && dh.Tag != 0xff {
			return newDescriptorUserDefined(i, dh, offsetEnd)
		}
		return newDescriptorUnknown(i, dh, offsetEnd)
	}
}
