package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/asticode/go-astikit"
)

type DescriptorTag uint8

// Descriptor tags
// Chapter: 6.1 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
const (
	DescriptorTagAC3                        DescriptorTag = 0x6a
	DescriptorTagAVCVideo                   DescriptorTag = 0x28
	DescriptorTagComponent                  DescriptorTag = 0x50
	DescriptorTagContent                    DescriptorTag = 0x54
	DescriptorTagDataStreamAlignment        DescriptorTag = 0x6
	DescriptorTagEnhancedAC3                DescriptorTag = 0x7a
	DescriptorTagExtendedEvent              DescriptorTag = 0x4e
	DescriptorTagExtension                  DescriptorTag = 0x7f
	DescriptorTagISO639LanguageAndAudioType DescriptorTag = 0xa
	DescriptorTagLocalTimeOffset            DescriptorTag = 0x58
	DescriptorTagMaximumBitrate             DescriptorTag = 0xe
	DescriptorTagNetworkName                DescriptorTag = 0x40
	DescriptorTagParentalRating             DescriptorTag = 0x55
	DescriptorTagPrivateDataIndicator       DescriptorTag = 0xf
	DescriptorTagPrivateDataSpecifier       DescriptorTag = 0x5f
	DescriptorTagRegistration               DescriptorTag = 0x5
	DescriptorTagService                    DescriptorTag = 0x48
	DescriptorTagShortEvent                 DescriptorTag = 0x4d
	DescriptorTagStreamIdentifier           DescriptorTag = 0x52
	DescriptorTagSubtitling                 DescriptorTag = 0x59
	DescriptorTagTeletext                   DescriptorTag = 0x56
	DescriptorTagVBIData                    DescriptorTag = 0x45
	DescriptorTagVBITeletext                DescriptorTag = 0x46
)

type DescriptorParser func(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (d Descriptor, err error)

var descriptorParserLUT = [256]DescriptorParser{
	DescriptorTagAC3:                        newDescriptorAC3,
	DescriptorTagAVCVideo:                   newDescriptorAVCVideo,
	DescriptorTagComponent:                  newDescriptorComponent,
	DescriptorTagContent:                    newDescriptorContent,
	DescriptorTagDataStreamAlignment:        newDescriptorDataStreamAlignment,
	DescriptorTagEnhancedAC3:                newDescriptorEnhancedAC3,
	DescriptorTagExtendedEvent:              newDescriptorExtendedEvent,
	DescriptorTagExtension:                  newDescriptorExtension,
	DescriptorTagISO639LanguageAndAudioType: newDescriptorISO639LanguageAndAudioType,
	DescriptorTagLocalTimeOffset:            newDescriptorLocalTimeOffset,
	DescriptorTagMaximumBitrate:             newDescriptorMaximumBitrate,
	DescriptorTagNetworkName:                newDescriptorNetworkName,
	DescriptorTagParentalRating:             newDescriptorParentalRating,
	DescriptorTagPrivateDataIndicator:       newDescriptorPrivateDataIndicator,
	DescriptorTagPrivateDataSpecifier:       newDescriptorPrivateDataSpecifier,
	DescriptorTagRegistration:               newDescriptorRegistration,
	DescriptorTagService:                    newDescriptorService,
	DescriptorTagShortEvent:                 newDescriptorShortEvent,
	DescriptorTagStreamIdentifier:           newDescriptorStreamIdentifier,
	DescriptorTagSubtitling:                 newDescriptorSubtitling,
	DescriptorTagTeletext:                   newDescriptorTeletext,
	DescriptorTagVBIData:                    newDescriptorVBIData,
	DescriptorTagVBITeletext:                newDescriptorTeletext,
}

func init() {
	for i := range descriptorParserLUT {
		if i&0x80 > 0 && i != 0xff {
			descriptorParserLUT[i] = newDescriptorUserDefined
			continue
		}
		if descriptorParserLUT[i] == nil {
			descriptorParserLUT[i] = newDescriptorUnknown
		}
	}
}

// ParseDescriptors parses descriptors
func ParseDescriptors(i *astikit.BytesIterator) (o []Descriptor, err error) {
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

			h := DescriptorHeader{
				Tag:    DescriptorTag(bs[0]),
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
			}
		}
	}
	return
}

func WriteDescriptorsWithLength(w *astikit.BitsWriter, ds []Descriptor) (int, error) {
	length := CalcDescriptorsLength(ds)
	b := astikit.NewBitsWriterBatch(w)

	b.Write(length | 0xf000)
	if err := b.Err(); err != nil {
		return 0, err
	}

	written, err := writeDescriptors(w, ds)
	return written + 2, err // 2 for length
}

func CalcDescriptorsLength(ds []Descriptor) (length uint16) {
	for _, d := range ds {
		length += 2 // tag and length
		length += uint16(d.length())
	}
	return
}

func writeDescriptors(w *astikit.BitsWriter, ds []Descriptor) (int, error) {
	written := 0

	for _, d := range ds {
		n, err := d.write(w)
		if err != nil {
			return 0, err
		}
		written += n
	}

	return written, nil
}

type Descriptor interface {
	length() uint8
	write(w *astikit.BitsWriter) (int, error)
}

type DescriptorHeader struct {
	Tag    DescriptorTag // the tag defines the structure of the contained data following the descriptor length.
	Length uint8
}

func (dh DescriptorHeader) parseDescriptor(i *astikit.BytesIterator, offsetEnd int) (d Descriptor, err error) {
	return descriptorParserLUT[dh.Tag](i, dh, offsetEnd)
}
