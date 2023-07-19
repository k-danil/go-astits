package astits

import (
	"encoding/binary"
	"fmt"
	"github.com/asticode/go-astikit"
	"time"
)

// Audio types
// Page: 683 | https://books.google.fr/books?id=6dgWB3-rChYC&printsec=frontcover&hl=fr
const (
	AudioTypeCleanEffects             = 0x1
	AudioTypeHearingImpaired          = 0x2
	AudioTypeVisualImpairedCommentary = 0x3
)

// Data stream alignments
// Page: 85 | Chapter:2.6.11 | Link: http://ecee.colorado.edu/~ecen5653/ecen5653/papers/iso13818-1.pdf
const (
	DataStreamAligmentAudioSyncWord          = 0x1
	DataStreamAligmentVideoSliceOrAccessUnit = 0x1
	DataStreamAligmentVideoAccessUnit        = 0x2
	DataStreamAligmentVideoGOPOrSEQ          = 0x3
	DataStreamAligmentVideoSEQ               = 0x4
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

// Descriptor extension tags
// Chapter: 6.3 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
const (
	DescriptorTagExtensionSupplementaryAudio = 0x6
)

// Service types
// Chapter: 6.2.33 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
const (
	ServiceTypeDigitalTelevisionService = 0x1
)

// Teletext types
// Chapter: 6.2.43 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
const (
	TeletextTypeAdditionalInformationPage                    = 0x3
	TeletextTypeInitialTeletextPage                          = 0x1
	TeletextTypeProgramSchedulePage                          = 0x4
	TeletextTypeTeletextSubtitlePage                         = 0x2
	TeletextTypeTeletextSubtitlePageForHearingImpairedPeople = 0x5
)

// VBI data service id
// Chapter: 6.2.47 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
const (
	VBIDataServiceIDEBUTeletext          = 0x1
	VBIDataServiceIDInvertedTeletext     = 0x2
	VBIDataServiceIDVPS                  = 0x4
	VBIDataServiceIDWSS                  = 0x5
	VBIDataServiceIDClosedCaptioning     = 0x6
	VBIDataServiceIDMonochrome442Samples = 0x7
)

// parseDescriptors parses descriptors
func parseDescriptors(i *astikit.BytesIterator) (o []Descriptor, err error) {
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

func writeDescriptorsWithLength(w *astikit.BitsWriter, ds []Descriptor) (int, error) {
	length := calcDescriptorsLength(ds)
	b := astikit.NewBitsWriterBatch(w)

	b.Write(length | 0xf000)
	if err := b.Err(); err != nil {
		return 0, err
	}

	written, err := writeDescriptors(w, ds)
	return written + 2, err // 2 for length
}

func calcDescriptorsLength(ds []Descriptor) (length uint16) {
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

func newDescriptorUserDefined(i *astikit.BytesIterator, h DescriptorHeader, _ int) (dd Descriptor, err error) {
	d := &DescriptorUserDefined{
		Header: h,
	}
	dd = d
	d.Data, err = i.NextBytes(int(h.Length))
	return
}

type DescriptorUserDefined struct {
	Header DescriptorHeader
	Data   []byte
}

func (d *DescriptorUserDefined) length() uint8 {
	return uint8(len(d.Data))
}

func (d *DescriptorUserDefined) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.Data)

	return written, b.Err()
}

// DescriptorAC3 represents an AC3 descriptor
// Chapter: Annex D | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorAC3 struct {
	Header           DescriptorHeader
	AdditionalInfo   []byte
	ASVC             uint8
	BSID             uint8
	ComponentType    uint8
	HasASVC          bool
	HasBSID          bool
	HasComponentType bool
	HasMainID        bool
	MainID           uint8
}

func newDescriptorAC3(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (dd Descriptor, err error) {
	// Get next byte
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Create descriptor
	d := &DescriptorAC3{
		Header:           h,
		HasASVC:          uint8(b&0x10) > 0,
		HasBSID:          uint8(b&0x40) > 0,
		HasComponentType: uint8(b&0x80) > 0,
		HasMainID:        uint8(b&0x20) > 0,
	}
	dd = d

	// Component type
	if d.HasComponentType {
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.ComponentType = b
	}

	// BSID
	if d.HasBSID {
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.BSID = b
	}

	// Main ID
	if d.HasMainID {
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.MainID = b
	}

	// ASVC
	if d.HasASVC {
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.ASVC = b
	}

	// Additional info
	if i.Offset() < offsetEnd {
		if d.AdditionalInfo, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *DescriptorAC3) length() (ret uint8) {
	ret = 1 // flags
	ret += b2u(d.HasComponentType)
	ret += b2u(d.HasBSID)
	ret += b2u(d.HasMainID)
	ret += b2u(d.HasASVC)
	ret += uint8(len(d.AdditionalInfo))

	return ret
}

func (d *DescriptorAC3) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.HasComponentType)
	b.Write(d.HasBSID)
	b.Write(d.HasMainID)
	b.Write(d.HasASVC)
	b.WriteN(uint8(0xff), 4)

	if d.HasComponentType {
		b.Write(d.ComponentType)
	}
	if d.HasBSID {
		b.Write(d.BSID)
	}
	if d.HasMainID {
		b.Write(d.MainID)
	}
	if d.HasASVC {
		b.Write(d.ASVC)
	}
	b.Write(d.AdditionalInfo)

	return written, b.Err()
}

// DescriptorAVCVideo represents an AVC video descriptor
// No doc found unfortunately, basing the implementation on https://github.com/gfto/bitstream/blob/master/mpeg/psi/desc_28.h
type DescriptorAVCVideo struct {
	Header               DescriptorHeader
	AVC24HourPictureFlag bool
	AVCStillPresent      bool
	CompatibleFlags      uint8
	ConstraintSet0Flag   bool
	ConstraintSet1Flag   bool
	ConstraintSet2Flag   bool
	LevelIDC             uint8
	ProfileIDC           uint8
}

func newDescriptorAVCVideo(i *astikit.BytesIterator, h DescriptorHeader, _ int) (dd Descriptor, err error) {
	// Init
	d := &DescriptorAVCVideo{
		Header: h,
	}
	dd = d

	// Get next byte
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Profile idc
	d.ProfileIDC = b

	// Get next byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Flags
	d.ConstraintSet0Flag = b&0x80 > 0
	d.ConstraintSet1Flag = b&0x40 > 0
	d.ConstraintSet2Flag = b&0x20 > 0
	d.CompatibleFlags = b & 0x1f

	// Get next byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Level idc
	d.LevelIDC = b

	// Get next byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// AVC still present
	d.AVCStillPresent = b&0x80 > 0

	// AVC 24 hour picture flag
	d.AVC24HourPictureFlag = b&0x40 > 0
	return
}

func (*DescriptorAVCVideo) length() uint8 {
	return 4
}

func (d *DescriptorAVCVideo) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.ProfileIDC)

	b.Write(d.ConstraintSet0Flag)
	b.Write(d.ConstraintSet1Flag)
	b.Write(d.ConstraintSet2Flag)
	b.WriteN(d.CompatibleFlags, 5)

	b.Write(d.LevelIDC)

	b.Write(d.AVCStillPresent)
	b.Write(d.AVC24HourPictureFlag)
	b.WriteN(uint8(0xff), 6)

	return written, b.Err()
}

// DescriptorComponent represents a component descriptor
// Chapter: 6.2.8 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorComponent struct {
	Header             DescriptorHeader
	ISO639LanguageCode [3]byte
	Text               []byte
	ComponentTag       uint8
	ComponentType      uint8
	StreamContent      uint8
	StreamContentExt   uint8
}

func newDescriptorComponent(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (dd Descriptor, err error) {
	// Init
	d := &DescriptorComponent{
		Header: h,
	}
	dd = d

	// Get next byte
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Stream content ext
	d.StreamContentExt = b >> 4

	// Stream content
	d.StreamContent = b & 0xf

	// Get next byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Component type
	d.ComponentType = b

	// Get next byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Component tag
	d.ComponentTag = b

	// ISO639 language code
	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	copy(d.ISO639LanguageCode[:], bs)

	// Text
	if i.Offset() < offsetEnd {
		if d.Text, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *DescriptorComponent) length() uint8 {
	return uint8(6 + len(d.Text))
}

func (d *DescriptorComponent) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.WriteN(d.StreamContentExt, 4)
	b.WriteN(d.StreamContent, 4)

	b.Write(d.ComponentType)
	b.Write(d.ComponentTag)

	b.Write(d.ISO639LanguageCode[:])

	b.Write(d.Text)

	return written, b.Err()
}

// DescriptorContent represents a content descriptor
// Chapter: 6.2.9 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorContent struct {
	Header DescriptorHeader
	Items  []DescriptorContentItem
}

// DescriptorContentItem represents a content item descriptor
// Chapter: 6.2.9 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorContentItem struct {
	ContentNibbleLevel1 uint8
	ContentNibbleLevel2 uint8
	UserByte            uint8
}

func newDescriptorContent(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (dd Descriptor, err error) {
	// Init
	d := &DescriptorContent{
		Header: h,
	}
	dd = d

	// Add items
	for i.Offset() < offsetEnd {
		// Get next bytes
		var bs []byte
		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}

		// Append item
		d.Items = append(d.Items, DescriptorContentItem{
			ContentNibbleLevel1: bs[0] >> 4,
			ContentNibbleLevel2: bs[0] & 0xf,
			UserByte:            bs[1],
		})
	}
	return
}

func (d *DescriptorContent) length() uint8 {
	return uint8(2 * len(d.Items))
}

func (d *DescriptorContent) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	for _, item := range d.Items {
		b.WriteN(item.ContentNibbleLevel1, 4)
		b.WriteN(item.ContentNibbleLevel2, 4)
		b.Write(item.UserByte)
	}

	return written, b.Err()
}

// DescriptorDataStreamAlignment represents a data stream alignment descriptor
type DescriptorDataStreamAlignment struct {
	Header DescriptorHeader
	Type   uint8
}

func newDescriptorDataStreamAlignment(i *astikit.BytesIterator, h DescriptorHeader, _ int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d := &DescriptorDataStreamAlignment{
		Header: h,
		Type:   b,
	}
	dd = d
	return
}

func (*DescriptorDataStreamAlignment) length() uint8 {
	return 1
}

func (d *DescriptorDataStreamAlignment) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.Type)

	return written, b.Err()
}

// DescriptorEnhancedAC3 represents an enhanced AC3 descriptor
// Chapter: Annex D | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorEnhancedAC3 struct {
	Header           DescriptorHeader
	AdditionalInfo   []byte
	ASVC             uint8
	BSID             uint8
	ComponentType    uint8
	HasASVC          bool
	HasBSID          bool
	HasComponentType bool
	HasMainID        bool
	HasSubStream1    bool
	HasSubStream2    bool
	HasSubStream3    bool
	MainID           uint8
	MixInfoExists    bool
	SubStream1       uint8
	SubStream2       uint8
	SubStream3       uint8
}

func newDescriptorEnhancedAC3(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (dd Descriptor, err error) {
	// Get next byte
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Create descriptor
	d := &DescriptorEnhancedAC3{
		Header:           h,
		HasASVC:          uint8(b&0x10) > 0,
		HasBSID:          uint8(b&0x40) > 0,
		HasComponentType: uint8(b&0x80) > 0,
		HasMainID:        uint8(b&0x20) > 0,
		HasSubStream1:    uint8(b&0x4) > 0,
		HasSubStream2:    uint8(b&0x2) > 0,
		HasSubStream3:    uint8(b&0x1) > 0,
		MixInfoExists:    uint8(b&0x8) > 0,
	}
	dd = d

	// Component type
	if d.HasComponentType {
		// Get next byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.ComponentType = b
	}

	// BSID
	if d.HasBSID {
		// Get next byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.BSID = b
	}

	// Main ID
	if d.HasMainID {
		// Get next byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.MainID = b
	}

	// ASVC
	if d.HasASVC {
		// Get next byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.ASVC = b
	}

	// Substream 1
	if d.HasSubStream1 {
		// Get next byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.SubStream1 = b
	}

	// Substream 2
	if d.HasSubStream2 {
		// Get next byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.SubStream2 = b
	}

	// Substream 3
	if d.HasSubStream3 {
		// Get next byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.SubStream3 = b
	}

	// Additional info
	if i.Offset() < offsetEnd {
		if d.AdditionalInfo, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *DescriptorEnhancedAC3) length() (ret uint8) {
	ret = 1 // flags
	ret += b2u(d.HasComponentType)
	ret += b2u(d.HasBSID)
	ret += b2u(d.HasMainID)
	ret += b2u(d.HasASVC)
	ret += b2u(d.HasSubStream1)
	ret += b2u(d.HasSubStream2)
	ret += b2u(d.HasSubStream3)
	ret += uint8(len(d.AdditionalInfo))

	return
}

func (d *DescriptorEnhancedAC3) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.HasComponentType)
	b.Write(d.HasBSID)
	b.Write(d.HasMainID)
	b.Write(d.HasASVC)
	b.Write(d.MixInfoExists)
	b.Write(d.HasSubStream1)
	b.Write(d.HasSubStream2)
	b.Write(d.HasSubStream3)

	if d.HasComponentType {
		b.Write(d.ComponentType)
	}
	if d.HasBSID {
		b.Write(d.BSID)
	}
	if d.HasMainID {
		b.Write(d.MainID)
	}
	if d.HasASVC {
		b.Write(d.ASVC)
	}
	if d.HasSubStream1 {
		b.Write(d.SubStream1)
	}
	if d.HasSubStream2 {
		b.Write(d.SubStream2)
	}
	if d.HasSubStream3 {
		b.Write(d.SubStream3)
	}

	b.Write(d.AdditionalInfo)

	return written, b.Err()
}

// DescriptorExtendedEvent represents an extended event descriptor
// Chapter: 6.2.15 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorExtendedEvent struct {
	Text                 []byte
	Items                []DescriptorExtendedEventItem
	Header               DescriptorHeader
	ISO639LanguageCode   [3]byte
	LastDescriptorNumber uint8
	Number               uint8
}

// DescriptorExtendedEventItem represents an extended event item descriptor
// Chapter: 6.2.15 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorExtendedEventItem struct {
	Content     []byte
	Description []byte
}

func newDescriptorExtendedEvent(i *astikit.BytesIterator, h DescriptorHeader, _ int) (dd Descriptor, err error) {
	// Init
	d := &DescriptorExtendedEvent{
		Header: h,
	}
	dd = d

	// Get next byte
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Number
	d.Number = b >> 4

	// Last descriptor number
	d.LastDescriptorNumber = b & 0xf

	// ISO639 language code
	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	copy(d.ISO639LanguageCode[:], bs)

	// Get next byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Items length
	itemsLength := int(b)

	// Items
	offsetEnd := i.Offset() + itemsLength
	for i.Offset() < offsetEnd {
		// Create item
		var item DescriptorExtendedEventItem
		if err = item.newDescriptorExtendedEventItem(i); err != nil {
			err = fmt.Errorf("astits: creating extended event item failed: %w", err)
			return
		}

		// Append item
		d.Items = append(d.Items, item)
	}

	// Get next byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Text length
	textLength := int(b)

	// Text
	if d.Text, err = i.NextBytes(textLength); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *DescriptorExtendedEventItem) newDescriptorExtendedEventItem(i *astikit.BytesIterator) (err error) {
	// Get next byte
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Description length
	descriptionLength := int(b)

	// Description
	if d.Description, err = i.NextBytes(descriptionLength); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	// Get next byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Content length
	contentLength := int(b)

	// Content
	if d.Content, err = i.NextBytes(contentLength); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *DescriptorExtendedEvent) length() uint8 {
	ret := 1 + 3 + 1 // numbers, language and items length

	itemsRet := 0
	for _, item := range d.Items {
		itemsRet += 1 // description length
		itemsRet += len(item.Description)
		itemsRet += 1 // content length
		itemsRet += len(item.Content)
	}

	ret += itemsRet

	ret += 1 // text length
	ret += len(d.Text)

	return uint8(ret)
}

func (d *DescriptorExtendedEvent) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	var lengthOfItems int
	for _, item := range d.Items {
		lengthOfItems += 1 // description length
		lengthOfItems += len(item.Description)
		lengthOfItems += 1 // content length
		lengthOfItems += len(item.Content)
	}

	b.WriteN(d.Number, 4)
	b.WriteN(d.LastDescriptorNumber, 4)

	b.Write(d.ISO639LanguageCode[:])

	b.Write(uint8(lengthOfItems))
	for _, item := range d.Items {
		b.Write(uint8(len(item.Description)))
		b.Write(item.Description)
		b.Write(uint8(len(item.Content)))
		b.Write(item.Content)
	}

	b.Write(uint8(len(d.Text)))
	b.Write(d.Text)

	return written, b.Err()
}

// DescriptorExtension represents an extension descriptor
// Chapter: 6.2.16 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorExtension struct {
	Header             DescriptorHeader
	SupplementaryAudio *DescriptorExtensionSupplementaryAudio
	Unknown            []byte
	Tag                uint8
}

func newDescriptorExtension(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (dd Descriptor, err error) {
	// Get next byte
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Create descriptor
	d := &DescriptorExtension{
		Header: h,
		Tag:    b,
	}
	dd = d

	// Switch on tag
	switch d.Tag {
	case DescriptorTagExtensionSupplementaryAudio:
		if d.SupplementaryAudio, err = newDescriptorExtensionSupplementaryAudio(i, offsetEnd); err != nil {
			err = fmt.Errorf("astits: parsing extension supplementary audio descriptor failed: %w", err)
			return
		}
	default:
		if d.Unknown, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *DescriptorExtension) length() uint8 {
	ret := 1 // tag

	switch d.Tag {
	case DescriptorTagExtensionSupplementaryAudio:
		ret += d.SupplementaryAudio.length()
	default:
		ret += len(d.Unknown)
	}

	return uint8(ret)
}

func (d *DescriptorExtension) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.Tag)

	switch d.Tag {
	case DescriptorTagExtensionSupplementaryAudio:
		err := d.SupplementaryAudio.write(w)
		if err != nil {
			return 0, err
		}
	default:
		if len(d.Unknown) > 0 {
			b.Write(d.Unknown)
		}
	}

	return written, b.Err()
}

// DescriptorExtensionSupplementaryAudio represents a supplementary audio extension descriptor
// Chapter: 6.4.10 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorExtensionSupplementaryAudio struct {
	LanguageCode            [3]byte
	PrivateData             []byte
	EditorialClassification uint8
	HasLanguageCode         bool
	MixType                 bool
}

func newDescriptorExtensionSupplementaryAudio(i *astikit.BytesIterator, offsetEnd int) (d *DescriptorExtensionSupplementaryAudio, err error) {
	// Get next byte
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Init
	d = &DescriptorExtensionSupplementaryAudio{
		EditorialClassification: b >> 2 & 0x1f,
		HasLanguageCode:         b&0x1 > 0,
		MixType:                 b&0x80 > 0,
	}

	// Language code
	if d.HasLanguageCode {
		var bs []byte
		if bs, err = i.NextBytesNoCopy(3); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		copy(d.LanguageCode[:], bs)
	}

	// Private data
	if i.Offset() < offsetEnd {
		if d.PrivateData, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *DescriptorExtensionSupplementaryAudio) length() int {
	ret := 1
	ret += 3 * int(b2u(d.HasLanguageCode))
	ret += len(d.PrivateData)
	return ret
}

func (d *DescriptorExtensionSupplementaryAudio) write(w *astikit.BitsWriter) error {
	b := astikit.NewBitsWriterBatch(w)

	b.Write(d.MixType)
	b.WriteN(d.EditorialClassification, 5)
	b.Write(true) // reserved
	b.Write(d.HasLanguageCode)

	if d.HasLanguageCode {
		b.Write(d.LanguageCode[:])
	}

	b.Write(d.PrivateData)

	return b.Err()
}

// DescriptorISO639LanguageAndAudioType represents an ISO639 language descriptor
// https://github.com/gfto/bitstream/blob/master/mpeg/psi/desc_0a.h
// FIXME (barbashov) according to Chapter 2.6.18 ISO/IEC 13818-1:2015 there could be not one, but multiple such descriptors
type DescriptorISO639LanguageAndAudioType struct {
	Language [3]byte
	Header   DescriptorHeader
	Type     uint8
}

// In some actual cases, the length is 3 and the language is described in only 2 bytes
//

func newDescriptorISO639LanguageAndAudioType(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (dd Descriptor, err error) {
	// Get next bytes
	var bs []byte
	if bs, err = i.NextBytesNoCopy(offsetEnd - i.Offset()); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	// Create descriptor
	d := &DescriptorISO639LanguageAndAudioType{
		Header: h,
		Type:   bs[len(bs)-1],
	}
	copy(d.Language[:], bs)
	dd = d
	return
}

func (*DescriptorISO639LanguageAndAudioType) length() uint8 {
	return 3 + 1 // language code + type
}

func (d *DescriptorISO639LanguageAndAudioType) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.Language[:])
	b.Write(d.Type)

	return written, b.Err()
}

// DescriptorLocalTimeOffset represents a local time offset descriptor
// Chapter: 6.2.20 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorLocalTimeOffset struct {
	Header DescriptorHeader
	Items  []DescriptorLocalTimeOffsetItem
}

// DescriptorLocalTimeOffsetItem represents a local time offset item descriptor
// Chapter: 6.2.20 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorLocalTimeOffsetItem struct {
	LocalTimeOffset         time.Duration
	NextTimeOffset          time.Duration
	TimeOfChange            time.Time
	CountryCode             [3]byte
	CountryRegionID         uint8
	LocalTimeOffsetPolarity bool
}

func newDescriptorLocalTimeOffset(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (dd Descriptor, err error) {
	// Init
	d := &DescriptorLocalTimeOffset{
		Header: h,
		Items:  make([]DescriptorLocalTimeOffsetItem, (offsetEnd-i.Offset())/13),
	}
	dd = d

	// Add items
	for idx := range d.Items {
		// Country code
		var bs []byte
		if bs, err = i.NextBytesNoCopy(3); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		copy(d.Items[idx].CountryCode[:], bs)

		// Get next byte
		var b byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		// Country region ID
		d.Items[idx].CountryRegionID = b >> 2

		// Local time offset polarity
		d.Items[idx].LocalTimeOffsetPolarity = b&0x1 > 0

		// Local time offset
		if d.Items[idx].LocalTimeOffset, err = parseDVBDurationMinutes(i); err != nil {
			err = fmt.Errorf("astits: parsing DVB durationminutes failed: %w", err)
			return
		}

		// Time of change
		if d.Items[idx].TimeOfChange, err = parseDVBTime(i); err != nil {
			err = fmt.Errorf("astits: parsing DVB time failed: %w", err)
			return
		}

		// Next time offset
		if d.Items[idx].NextTimeOffset, err = parseDVBDurationMinutes(i); err != nil {
			err = fmt.Errorf("astits: parsing DVB duration minutes failed: %w", err)
			return
		}
	}
	return
}

func (d *DescriptorLocalTimeOffset) length() uint8 {
	return uint8(13 * len(d.Items))
}

func (d *DescriptorLocalTimeOffset) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	for _, item := range d.Items {
		b.Write(item.CountryCode[:])

		b.WriteN(item.CountryRegionID, 6)
		b.WriteN(uint8(0xff), 1)
		b.Write(item.LocalTimeOffsetPolarity)

		if _, err := writeDVBDurationMinutes(w, item.LocalTimeOffset); err != nil {
			return 0, err
		}
		if _, err := writeDVBTime(w, item.TimeOfChange); err != nil {
			return 0, err
		}
		if _, err := writeDVBDurationMinutes(w, item.NextTimeOffset); err != nil {
			return 0, err
		}
	}

	return written, b.Err()
}

// DescriptorMaximumBitrate represents a maximum bitrate descriptor
type DescriptorMaximumBitrate struct {
	Bitrate uint32 // In bytes/second
	Header  DescriptorHeader
}

func newDescriptorMaximumBitrate(i *astikit.BytesIterator, h DescriptorHeader, _ int) (dd Descriptor, err error) {
	// Get next bytes
	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	// Create descriptor
	d := &DescriptorMaximumBitrate{
		Header:  h,
		Bitrate: (uint32(bs[0]&0x3f)<<16 | uint32(bs[1])<<8 | uint32(bs[2])) * 50,
	}
	dd = d
	return
}

func (*DescriptorMaximumBitrate) length() uint8 {
	return 3
}

func (d *DescriptorMaximumBitrate) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.WriteN(uint8(0xff), 2)
	b.WriteN(d.Bitrate/50, 22)

	return written, b.Err()
}

// DescriptorNetworkName represents a network name descriptor
// Chapter: 6.2.27 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorNetworkName struct {
	Header DescriptorHeader
	Name   []byte
}

func newDescriptorNetworkName(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (dd Descriptor, err error) {
	// Create descriptor
	d := &DescriptorNetworkName{
		Header: h,
	}
	dd = d

	// Name
	if d.Name, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *DescriptorNetworkName) length() uint8 {
	return uint8(len(d.Name))
}

func (d *DescriptorNetworkName) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.Name)

	return written, b.Err()
}

// DescriptorParentalRating represents a parental rating descriptor
// Chapter: 6.2.28 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorParentalRating struct {
	Header DescriptorHeader
	Items  []DescriptorParentalRatingItem
}

// DescriptorParentalRatingItem represents a parental rating item descriptor
// Chapter: 6.2.28 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorParentalRatingItem struct {
	CountryCode [3]byte
	Rating      uint8
}

// MinimumAge returns the minimum age for the parental rating
func (d DescriptorParentalRatingItem) MinimumAge() int {
	// Undefined or user defined ratings
	if d.Rating == 0 || d.Rating > 0x10 {
		return 0
	}
	return int(d.Rating) + 3
}

func newDescriptorParentalRating(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (dd Descriptor, err error) {
	// Create descriptor
	d := &DescriptorParentalRating{
		Header: h,
		Items:  make([]DescriptorParentalRatingItem, (offsetEnd-i.Offset())/4),
	}
	dd = d

	// Add items
	for idx := range d.Items {
		// Get next bytes
		var bs []byte
		if bs, err = i.NextBytesNoCopy(4); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		d.Items[idx].Rating = bs[3]
		copy(d.Items[idx].CountryCode[:], bs)
	}
	return
}

func (d *DescriptorParentalRating) length() uint8 {
	return uint8(4 * len(d.Items))
}

func (d *DescriptorParentalRating) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	for _, item := range d.Items {
		b.Write(item.CountryCode[:])
		b.Write(item.Rating)
	}

	return written, b.Err()
}

// DescriptorPrivateDataIndicator represents a private data Indicator descriptor
type DescriptorPrivateDataIndicator struct {
	Header    DescriptorHeader
	Indicator uint32
}

func newDescriptorPrivateDataIndicator(i *astikit.BytesIterator, h DescriptorHeader, _ int) (dd Descriptor, err error) {
	// Get next bytes
	var bs []byte
	if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	// Create descriptor
	d := &DescriptorPrivateDataIndicator{
		Header:    h,
		Indicator: binary.BigEndian.Uint32(bs),
	}
	dd = d
	return
}

func (*DescriptorPrivateDataIndicator) length() uint8 {
	return 4
}

func (d *DescriptorPrivateDataIndicator) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.Indicator)

	return written, b.Err()
}

// DescriptorPrivateDataSpecifier represents a private data specifier descriptor
type DescriptorPrivateDataSpecifier struct {
	Header    DescriptorHeader
	Specifier uint32
}

func newDescriptorPrivateDataSpecifier(i *astikit.BytesIterator, h DescriptorHeader, _ int) (dd Descriptor, err error) {
	// Get next bytes
	var bs []byte
	if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	// Create descriptor
	d := &DescriptorPrivateDataSpecifier{
		Header:    h,
		Specifier: binary.BigEndian.Uint32(bs),
	}
	dd = d
	return
}

func (*DescriptorPrivateDataSpecifier) length() uint8 {
	return 4
}

func (d *DescriptorPrivateDataSpecifier) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.Specifier)

	return written, b.Err()
}

// DescriptorRegistration represents a registration descriptor
// Page: 84 | http://ecee.colorado.edu/~ecen5653/ecen5653/papers/iso13818-1.pdf
type DescriptorRegistration struct {
	AdditionalIdentificationInfo []byte
	FormatIdentifier             uint32
	Header                       DescriptorHeader
}

func newDescriptorRegistration(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (dd Descriptor, err error) {
	// Get next bytes
	var bs []byte
	if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	// Create descriptor
	d := &DescriptorRegistration{
		Header:           h,
		FormatIdentifier: binary.BigEndian.Uint32(bs),
	}
	dd = d

	// Additional identification info
	if i.Offset() < offsetEnd {
		if d.AdditionalIdentificationInfo, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *DescriptorRegistration) length() uint8 {
	return uint8(4 + len(d.AdditionalIdentificationInfo))
}

func (d *DescriptorRegistration) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.FormatIdentifier)
	b.Write(d.AdditionalIdentificationInfo)

	return written, b.Err()
}

// DescriptorService represents a service descriptor
// Chapter: 6.2.33 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorService struct {
	Name     []byte
	Provider []byte
	Header   DescriptorHeader
	Type     uint8
}

func newDescriptorService(i *astikit.BytesIterator, h DescriptorHeader, _ int) (dd Descriptor, err error) {
	// Get next byte
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Create descriptor
	d := &DescriptorService{
		Header: h,
		Type:   b,
	}
	dd = d

	// Get next byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Provider length
	providerLength := int(b)

	// Provider
	if d.Provider, err = i.NextBytes(providerLength); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	// Get next byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Name length
	nameLength := int(b)

	// Name
	if d.Name, err = i.NextBytes(nameLength); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *DescriptorService) length() uint8 {
	ret := 3 // type and lengths
	ret += len(d.Name)
	ret += len(d.Provider)
	return uint8(ret)
}

func (d *DescriptorService) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.Type)
	b.Write(uint8(len(d.Provider)))
	b.Write(d.Provider)
	b.Write(uint8(len(d.Name)))
	b.Write(d.Name)

	return written, b.Err()
}

// DescriptorShortEvent represents a short event descriptor
// Chapter: 6.2.37 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorShortEvent struct {
	Header    DescriptorHeader
	EventName []byte
	Language  [3]byte
	Text      []byte
}

func newDescriptorShortEvent(i *astikit.BytesIterator, h DescriptorHeader, _ int) (dd Descriptor, err error) {
	// Create descriptor
	d := &DescriptorShortEvent{
		Header: h,
	}
	dd = d

	// Language
	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	copy(d.Language[:], bs)

	// Get next byte
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Event length
	eventLength := int(b)

	// Event name
	if d.EventName, err = i.NextBytes(eventLength); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	// Get next byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Text length
	textLength := int(b)

	// Text
	if d.Text, err = i.NextBytes(textLength); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *DescriptorShortEvent) length() uint8 {
	ret := 3 + 1 + 1 // language code and lengths
	ret += len(d.EventName)
	ret += len(d.Text)
	return uint8(ret)
}

func (d *DescriptorShortEvent) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.Language[:])

	b.Write(uint8(len(d.EventName)))
	b.Write(d.EventName)

	b.Write(uint8(len(d.Text)))
	b.Write(d.Text)

	return written, b.Err()
}

// DescriptorStreamIdentifier represents a stream identifier descriptor
// Chapter: 6.2.39 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorStreamIdentifier struct {
	Header       DescriptorHeader
	ComponentTag uint8
}

func newDescriptorStreamIdentifier(i *astikit.BytesIterator, h DescriptorHeader, _ int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d := &DescriptorStreamIdentifier{
		Header:       h,
		ComponentTag: b,
	}
	dd = d
	return
}

func (*DescriptorStreamIdentifier) length() uint8 {
	return 1
}

func (d *DescriptorStreamIdentifier) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.ComponentTag)

	return written, b.Err()
}

// DescriptorSubtitling represents a subtitling descriptor
// Chapter: 6.2.41 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorSubtitling struct {
	Header DescriptorHeader
	Items  []DescriptorSubtitlingItem
}

// DescriptorSubtitlingItem represents subtitling descriptor item
// Chapter: 6.2.41 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorSubtitlingItem struct {
	Language          [3]byte
	AncillaryPageID   uint16
	CompositionPageID uint16
	Type              uint8
}

func newDescriptorSubtitling(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (dd Descriptor, err error) {
	// Create descriptor
	d := &DescriptorSubtitling{
		Header: h,
		Items:  make([]DescriptorSubtitlingItem, (offsetEnd-i.Offset())/8),
	}
	dd = d

	// Loop
	for idx := range d.Items {
		// Language
		var bs []byte
		if bs, err = i.NextBytesNoCopy(3); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		copy(d.Items[idx].Language[:], bs)

		// Get next byte
		var b byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		// Type
		d.Items[idx].Type = b

		// Get next bytes
		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}

		// Composition page ID
		d.Items[idx].CompositionPageID = binary.BigEndian.Uint16(bs)

		// Get next bytes
		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}

		// Ancillary page ID
		d.Items[idx].AncillaryPageID = binary.BigEndian.Uint16(bs)
	}
	return
}

func (d *DescriptorSubtitling) length() uint8 {
	return uint8(8 * len(d.Items))
}

func (d *DescriptorSubtitling) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	for _, item := range d.Items {
		b.Write(item.Language[:])
		b.Write(item.Type)
		b.Write(item.CompositionPageID)
		b.Write(item.AncillaryPageID)
	}

	return written, b.Err()
}

// DescriptorTeletext represents a teletext descriptor
// Chapter: 6.2.43 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorTeletext struct {
	Header DescriptorHeader
	Items  []DescriptorTeletextItem
}

// DescriptorTeletextItem represents a teletext descriptor item
// Chapter: 6.2.43 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorTeletextItem struct {
	Language [3]byte
	Magazine uint8
	Page     uint8
	Type     uint8
}

func newDescriptorTeletext(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (dd Descriptor, err error) {
	// Create descriptor
	d := &DescriptorTeletext{
		Header: h,
		Items:  make([]DescriptorTeletextItem, (offsetEnd-i.Offset())/5),
	}
	dd = d

	// Loop
	for idx := range d.Items {
		// Language
		var bs []byte
		if bs, err = i.NextBytesNoCopy(3); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		copy(d.Items[idx].Language[:], bs)

		// Get next byte
		var b byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		// Type
		d.Items[idx].Type = b >> 3

		// Magazine
		d.Items[idx].Magazine = b & 0x7

		// Get next byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		// Page
		d.Items[idx].Page = b>>4*10 + b&0xf
	}
	return
}

func (d *DescriptorTeletext) length() uint8 {
	return uint8(5 * len(d.Items))
}

func (d *DescriptorTeletext) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	for _, item := range d.Items {
		b.Write(item.Language[:])
		b.WriteN(item.Type, 5)
		b.WriteN(item.Magazine, 3)
		b.WriteN(item.Page/10, 4)
		b.WriteN(item.Page%10, 4)
	}

	return written, b.Err()
}

// DescriptorVBIData represents a VBI data descriptor
// Chapter: 6.2.47 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorVBIData struct {
	Header   DescriptorHeader
	Services []DescriptorVBIDataService
}

// DescriptorVBIDataService represents a vbi data service descriptor
// Chapter: 6.2.47 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorVBIDataService struct {
	DataServiceID uint8
	Descriptors   []DescriptorVBIDataDescriptor
}

// DescriptorVBIDataItem represents a vbi data descriptor item
// Chapter: 6.2.47 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorVBIDataDescriptor struct {
	FieldParity bool
	LineOffset  uint8
}

func newDescriptorVBIData(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (dd Descriptor, err error) {
	// Create descriptor
	d := &DescriptorVBIData{
		Header:   h,
		Services: make([]DescriptorVBIDataService, (offsetEnd-i.Offset())/3),
	}
	dd = d

	// Loop
	for idx := range d.Services {
		// Get next byte
		var b byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		// Data service ID
		d.Services[idx].DataServiceID = b

		// Get next byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		// Data service descriptor length
		dataServiceDescriptorLength := int(b)

		// Data service descriptor
		offsetDataEnd := i.Offset() + dataServiceDescriptorLength
		for i.Offset() < offsetDataEnd {
			// Get next byte
			if b, err = i.NextByte(); err != nil {
				err = fmt.Errorf("astits: fetching next byte failed: %w", err)
				return
			}

			if d.Services[idx].DataServiceID <= VBIDataServiceIDMonochrome442Samples &&
				d.Services[idx].DataServiceID != 0x0 && d.Services[idx].DataServiceID != 0x3 {
				// Append data
				d.Services[idx].Descriptors = append(d.Services[idx].Descriptors, DescriptorVBIDataDescriptor{
					FieldParity: b&0x20 > 0,
					LineOffset:  b & 0x1f,
				})
			}
		}
	}
	return
}

func (d *DescriptorVBIData) length() uint8 {
	return uint8(3 * len(d.Services))
}

func (d *DescriptorVBIData) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	for _, item := range d.Services {
		b.Write(item.DataServiceID)

		if item.DataServiceID <= VBIDataServiceIDMonochrome442Samples &&
			item.DataServiceID != 0x0 && item.DataServiceID != 0x3 {

			b.Write(uint8(len(item.Descriptors))) // each descriptor is 1 byte
			for _, desc := range item.Descriptors {
				b.WriteN(uint8(0xff), 2)
				b.Write(desc.FieldParity)
				b.WriteN(desc.LineOffset, 5)
			}
		} else {
			// let'bs put one reserved byte
			b.Write(uint8(1))
			b.Write(uint8(0xff))
		}
	}

	return written, b.Err()
}

type DescriptorUnknown struct {
	Header  DescriptorHeader
	Content []byte
}

func newDescriptorUnknown(i *astikit.BytesIterator, h DescriptorHeader, _ int) (dd Descriptor, err error) {
	// Create descriptor
	d := &DescriptorUnknown{
		Header: h,
	}
	dd = d

	// Get next bytes
	if d.Content, err = i.NextBytes(int(h.Length)); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *DescriptorUnknown) length() uint8 {
	return uint8(len(d.Content))
}

func (d *DescriptorUnknown) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.Content)

	return written, b.Err()
}
