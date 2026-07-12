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
	TagAAC                          Tag = 0x7c
	TagAC3                          Tag = 0x6a
	TagAVCTimingAndHRD              Tag = 0x2a
	TagAVCVideo                     Tag = 0x28
	TagAdaptationFieldData          Tag = 0x70
	TagAncillaryData                Tag = 0x6b
	TagAnnouncementSupport          Tag = 0x6e
	TagAudioStream                  Tag = 0x3
	TagAuxiliaryVideoStream         Tag = 0x2f
	TagBouquetName                  Tag = 0x47
	TagCA                           Tag = 0x9
	TagCAIdentifier                 Tag = 0x53
	TagCableDeliverySystem          Tag = 0x44
	TagCellFrequencyLink            Tag = 0x6d
	TagCellList                     Tag = 0x6c
	TagComponent                    Tag = 0x50
	TagContent                      Tag = 0x54
	TagContentLabeling              Tag = 0x24
	TagCopyright                    Tag = 0xd
	TagCountryAvailability          Tag = 0x49
	TagDSNG                         Tag = 0x68
	TagDTS                          Tag = 0x7b
	TagDataBroadcast                Tag = 0x64
	TagDataBroadcastID              Tag = 0x66
	TagDataStreamAlignment          Tag = 0x6
	TagEnhancedAC3                  Tag = 0x7a
	TagExtendedEvent                Tag = 0x4e
	TagExtension                    Tag = 0x7f
	TagExternalESID                 Tag = 0x20
	TagFMC                          Tag = 0x1f
	TagFTAContentManagement         Tag = 0x7e
	TagFlexMuxTiming                Tag = 0x2c
	TagFmxBufferSize                Tag = 0x22
	TagFrequencyList                Tag = 0x62
	TagHEVCVideo                    Tag = 0x38
	TagHierarchy                    Tag = 0x4
	TagIBP                          Tag = 0x12
	TagIOD                          Tag = 0x1d
	TagISO639LanguageAndAudioType   Tag = 0xa
	TagJ2KVideo                     Tag = 0x32
	TagLinkage                      Tag = 0x4a
	TagLocalTimeOffset              Tag = 0x58
	TagMPEG2AACAudio                Tag = 0x2b
	TagMPEG2StereoscopicVideoFormat Tag = 0x34
	TagMPEG4Audio                   Tag = 0x1c
	TagMPEG4AudioExtension          Tag = 0x2e
	TagMPEG4Text                    Tag = 0x2d
	TagMPEG4Video                   Tag = 0x1b
	TagMPEGExtension                Tag = 0x3f
	TagMVCExtension                 Tag = 0x31
	TagMVCOperationPoint            Tag = 0x33
	TagMaximumBitrate               Tag = 0xe
	TagMetadata                     Tag = 0x26
	TagMetadataPointer              Tag = 0x25
	TagMetadataSTD                  Tag = 0x27
	TagMosaic                       Tag = 0x51
	TagMultilingualBouquetName      Tag = 0x5c
	TagMultilingualComponent        Tag = 0x5e
	TagMultilingualNetworkName      Tag = 0x5b
	TagMultilingualServiceName      Tag = 0x5d
	TagMultiplexBuffer              Tag = 0x23
	TagMultiplexBufferUtilization   Tag = 0xc
	TagMuxCode                      Tag = 0x21
	TagNVODReference                Tag = 0x4b
	TagNetworkName                  Tag = 0x40
	TagPDC                          Tag = 0x69
	TagParentalRating               Tag = 0x55
	TagPartialTransportStream       Tag = 0x63
	TagPrivateDataIndicator         Tag = 0xf
	TagPrivateDataSpecifier         Tag = 0x5f
	TagRegistration                 Tag = 0x5
	TagS2SatelliteDeliverySystem    Tag = 0x79
	TagSL                           Tag = 0x1e
	TagSTD                          Tag = 0x11
	TagSVCExtension                 Tag = 0x30
	TagSatelliteDeliverySystem      Tag = 0x43
	TagScrambling                   Tag = 0x65
	TagService                      Tag = 0x48
	TagServiceAvailability          Tag = 0x72
	TagServiceList                  Tag = 0x41
	TagServiceMove                  Tag = 0x60
	TagShortEvent                   Tag = 0x4d
	TagShortSmoothingBuffer         Tag = 0x61
	TagSmoothingBuffer              Tag = 0x10
	TagStereoscopicProgramInfo      Tag = 0x35
	TagStereoscopicVideoInfo        Tag = 0x36
	TagStreamIdentifier             Tag = 0x52
	TagStuffing                     Tag = 0x42
	TagSubtitling                   Tag = 0x59
	TagSystemClock                  Tag = 0xb
	TagTargetBackgroundGrid         Tag = 0x7
	TagTelephone                    Tag = 0x57
	TagTeletext                     Tag = 0x56
	TagTerrestrialDeliverySystem    Tag = 0x5a
	TagTimeShiftedEvent             Tag = 0x4f
	TagTimeShiftedService           Tag = 0x4c
	TagTransportProfile             Tag = 0x37
	TagTransportStream              Tag = 0x67
	TagVBIData                      Tag = 0x45
	TagVBITeletext                  Tag = 0x46
	TagVideoStream                  Tag = 0x2
	TagVideoWindow                  Tag = 0x8
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

// ParseN parses a descriptor loop of exactly length bytes at the start of bs,
// without a leading 2-byte length prefix — the form a CAT uses, where the loop
// is bounded by the section length instead.
func ParseN(bs []byte, length int) (ds []Descriptor, n int, err error) {
	i := bytesiter.New(bs)
	if ds, err = parseDescriptorsN(i, length); err != nil {
		return
	}
	return ds, i.Offset(), nil
}

func parseDescriptors(i *bytesiter.Iterator) (o []Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	length := int(binary.BigEndian.Uint16(bs) & 0xfff)
	return parseDescriptorsN(i, length)
}

func parseDescriptorsN(i *bytesiter.Iterator, length int) (o []Descriptor, err error) {
	if length > 0 {
		curOffset := i.Offset()
		offsetEnd := i.Offset() + length

		var bs []byte
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
			if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
				err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
				return
			}

			h := Header{
				Tag:    Tag(bs[0]),
				Length: bs[1],
			}

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

// Append appends the serialized descriptors with no length prefix; the caller
// bounds them by the section length, as CAT and TSDT do.
func Append(dst []byte, ds []Descriptor) []byte {
	for _, d := range ds {
		dst = d.Append(dst)
	}
	return dst
}

// AppendWithLength appends the 2-byte descriptors length prefix followed by
// the serialized descriptors.
func AppendWithLength(dst []byte, ds []Descriptor) []byte {
	length := uint16(CalcLength(ds))
	dst = append(dst, byte(length>>8)|0xf0, byte(length))
	return Append(dst, ds)
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
	case TagAAC:
		return newDescriptorAAC(i, dh, offsetEnd)
	case TagAC3:
		return newDescriptorAC3(i, dh, offsetEnd)
	case TagAVCVideo:
		return newDescriptorAVCVideo(i, dh, offsetEnd)
	case TagAdaptationFieldData:
		return newDescriptorAdaptationFieldData(i, dh, offsetEnd)
	case TagAncillaryData:
		return newDescriptorAncillaryData(i, dh, offsetEnd)
	case TagAnnouncementSupport:
		return newDescriptorAnnouncementSupport(i, dh, offsetEnd)
	case TagBouquetName:
		return newDescriptorBouquetName(i, dh, offsetEnd)
	case TagCA:
		return newDescriptorCA(i, dh, offsetEnd)
	case TagCAIdentifier:
		return newDescriptorCAIdentifier(i, dh, offsetEnd)
	case TagCableDeliverySystem:
		return newDescriptorCableDeliverySystem(i, dh, offsetEnd)
	case TagCellFrequencyLink:
		return newDescriptorCellFrequencyLink(i, dh, offsetEnd)
	case TagCellList:
		return newDescriptorCellList(i, dh, offsetEnd)
	case TagComponent:
		return newDescriptorComponent(i, dh, offsetEnd)
	case TagContent:
		return newDescriptorContent(i, dh, offsetEnd)
	case TagCountryAvailability:
		return newDescriptorCountryAvailability(i, dh, offsetEnd)
	case TagDSNG:
		return newDescriptorDSNG(i, dh, offsetEnd)
	case TagDTS:
		return newDescriptorDTS(i, dh, offsetEnd)
	case TagDataBroadcast:
		return newDescriptorDataBroadcast(i, dh, offsetEnd)
	case TagDataBroadcastID:
		return newDescriptorDataBroadcastID(i, dh, offsetEnd)
	case TagDataStreamAlignment:
		return newDescriptorDataStreamAlignment(i, dh, offsetEnd)
	case TagEnhancedAC3:
		return newDescriptorEnhancedAC3(i, dh, offsetEnd)
	case TagExtendedEvent:
		return newDescriptorExtendedEvent(i, dh, offsetEnd)
	case TagExtension:
		return newDescriptorExtension(i, dh, offsetEnd)
	case TagFTAContentManagement:
		return newDescriptorFTAContentManagement(i, dh, offsetEnd)
	case TagFrequencyList:
		return newDescriptorFrequencyList(i, dh, offsetEnd)
	case TagISO639LanguageAndAudioType:
		return newDescriptorISO639LanguageAndAudioType(i, dh, offsetEnd)
	case TagLinkage:
		return newDescriptorLinkage(i, dh, offsetEnd)
	case TagLocalTimeOffset:
		return newDescriptorLocalTimeOffset(i, dh, offsetEnd)
	case TagMaximumBitrate:
		return newDescriptorMaximumBitrate(i, dh, offsetEnd)
	case TagMosaic:
		return newDescriptorMosaic(i, dh, offsetEnd)
	case TagMultilingualBouquetName:
		return newDescriptorMultilingualBouquetName(i, dh, offsetEnd)
	case TagMultilingualComponent:
		return newDescriptorMultilingualComponent(i, dh, offsetEnd)
	case TagMultilingualNetworkName:
		return newDescriptorMultilingualNetworkName(i, dh, offsetEnd)
	case TagMultilingualServiceName:
		return newDescriptorMultilingualServiceName(i, dh, offsetEnd)
	case TagNVODReference:
		return newDescriptorNVODReference(i, dh, offsetEnd)
	case TagNetworkName:
		return newDescriptorNetworkName(i, dh, offsetEnd)
	case TagPDC:
		return newDescriptorPDC(i, dh, offsetEnd)
	case TagParentalRating:
		return newDescriptorParentalRating(i, dh, offsetEnd)
	case TagPartialTransportStream:
		return newDescriptorPartialTransportStream(i, dh, offsetEnd)
	case TagPrivateDataIndicator:
		return newDescriptorPrivateDataIndicator(i, dh, offsetEnd)
	case TagPrivateDataSpecifier:
		return newDescriptorPrivateDataSpecifier(i, dh, offsetEnd)
	case TagRegistration:
		return newDescriptorRegistration(i, dh, offsetEnd)
	case TagS2SatelliteDeliverySystem:
		return newDescriptorS2SatelliteDeliverySystem(i, dh, offsetEnd)
	case TagSatelliteDeliverySystem:
		return newDescriptorSatelliteDeliverySystem(i, dh, offsetEnd)
	case TagScrambling:
		return newDescriptorScrambling(i, dh, offsetEnd)
	case TagService:
		return newDescriptorService(i, dh, offsetEnd)
	case TagServiceAvailability:
		return newDescriptorServiceAvailability(i, dh, offsetEnd)
	case TagServiceList:
		return newDescriptorServiceList(i, dh, offsetEnd)
	case TagServiceMove:
		return newDescriptorServiceMove(i, dh, offsetEnd)
	case TagShortEvent:
		return newDescriptorShortEvent(i, dh, offsetEnd)
	case TagShortSmoothingBuffer:
		return newDescriptorShortSmoothingBuffer(i, dh, offsetEnd)
	case TagStreamIdentifier:
		return newDescriptorStreamIdentifier(i, dh, offsetEnd)
	case TagStuffing:
		return newDescriptorStuffing(i, dh, offsetEnd)
	case TagSubtitling:
		return newDescriptorSubtitling(i, dh, offsetEnd)
	case TagTelephone:
		return newDescriptorTelephone(i, dh, offsetEnd)
	case TagTeletext, TagVBITeletext:
		return newDescriptorTeletext(i, dh, offsetEnd)
	case TagTerrestrialDeliverySystem:
		return newDescriptorTerrestrialDeliverySystem(i, dh, offsetEnd)
	case TagTimeShiftedEvent:
		return newDescriptorTimeShiftedEvent(i, dh, offsetEnd)
	case TagTimeShiftedService:
		return newDescriptorTimeShiftedService(i, dh, offsetEnd)
	case TagTransportStream:
		return newDescriptorTransportStream(i, dh, offsetEnd)
	case TagVBIData:
		return newDescriptorVBIData(i, dh, offsetEnd)
	case TagAVCTimingAndHRD:
		return newDescriptorAVCTimingAndHRD(i, dh, offsetEnd)
	case TagAudioStream:
		return newDescriptorAudioStream(i, dh, offsetEnd)
	case TagAuxiliaryVideoStream:
		return newDescriptorAuxiliaryVideoStream(i, dh, offsetEnd)
	case TagContentLabeling:
		return newDescriptorContentLabeling(i, dh, offsetEnd)
	case TagCopyright:
		return newDescriptorCopyright(i, dh, offsetEnd)
	case TagExternalESID:
		return newDescriptorExternalESID(i, dh, offsetEnd)
	case TagFMC:
		return newDescriptorFMC(i, dh, offsetEnd)
	case TagFlexMuxTiming:
		return newDescriptorFlexMuxTiming(i, dh, offsetEnd)
	case TagFmxBufferSize:
		return newDescriptorFmxBufferSize(i, dh, offsetEnd)
	case TagHEVCVideo:
		return newDescriptorHEVCVideo(i, dh, offsetEnd)
	case TagHierarchy:
		return newDescriptorHierarchy(i, dh, offsetEnd)
	case TagIBP:
		return newDescriptorIBP(i, dh, offsetEnd)
	case TagIOD:
		return newDescriptorIOD(i, dh, offsetEnd)
	case TagJ2KVideo:
		return newDescriptorJ2KVideo(i, dh, offsetEnd)
	case TagMPEG2AACAudio:
		return newDescriptorMPEG2AACAudio(i, dh, offsetEnd)
	case TagMPEG2StereoscopicVideoFormat:
		return newDescriptorMPEG2StereoscopicVideoFormat(i, dh, offsetEnd)
	case TagMPEG4Audio:
		return newDescriptorMPEG4Audio(i, dh, offsetEnd)
	case TagMPEG4AudioExtension:
		return newDescriptorMPEG4AudioExtension(i, dh, offsetEnd)
	case TagMPEG4Text:
		return newDescriptorMPEG4Text(i, dh, offsetEnd)
	case TagMPEG4Video:
		return newDescriptorMPEG4Video(i, dh, offsetEnd)
	case TagMPEGExtension:
		return newDescriptorMPEGExtension(i, dh, offsetEnd)
	case TagMVCExtension:
		return newDescriptorMVCExtension(i, dh, offsetEnd)
	case TagMVCOperationPoint:
		return newDescriptorMVCOperationPoint(i, dh, offsetEnd)
	case TagMetadata:
		return newDescriptorMetadata(i, dh, offsetEnd)
	case TagMetadataPointer:
		return newDescriptorMetadataPointer(i, dh, offsetEnd)
	case TagMetadataSTD:
		return newDescriptorMetadataSTD(i, dh, offsetEnd)
	case TagMultiplexBuffer:
		return newDescriptorMultiplexBuffer(i, dh, offsetEnd)
	case TagMultiplexBufferUtilization:
		return newDescriptorMultiplexBufferUtilization(i, dh, offsetEnd)
	case TagMuxCode:
		return newDescriptorMuxCode(i, dh, offsetEnd)
	case TagSL:
		return newDescriptorSL(i, dh, offsetEnd)
	case TagSTD:
		return newDescriptorSTD(i, dh, offsetEnd)
	case TagSVCExtension:
		return newDescriptorSVCExtension(i, dh, offsetEnd)
	case TagSmoothingBuffer:
		return newDescriptorSmoothingBuffer(i, dh, offsetEnd)
	case TagStereoscopicProgramInfo:
		return newDescriptorStereoscopicProgramInfo(i, dh, offsetEnd)
	case TagStereoscopicVideoInfo:
		return newDescriptorStereoscopicVideoInfo(i, dh, offsetEnd)
	case TagSystemClock:
		return newDescriptorSystemClock(i, dh, offsetEnd)
	case TagTargetBackgroundGrid:
		return newDescriptorTargetBackgroundGrid(i, dh, offsetEnd)
	case TagTransportProfile:
		return newDescriptorTransportProfile(i, dh, offsetEnd)
	case TagVideoStream:
		return newDescriptorVideoStream(i, dh, offsetEnd)
	case TagVideoWindow:
		return newDescriptorVideoWindow(i, dh, offsetEnd)
	default:
		if dh.Tag >= userDefinedTagsStart && dh.Tag != 0xff {
			return newDescriptorUserDefined(i, dh, offsetEnd)
		}
		return newDescriptorUnknown(i, dh, offsetEnd)
	}
}
