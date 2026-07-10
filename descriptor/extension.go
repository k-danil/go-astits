package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

// Descriptor extension tags
// Chapter: 6.3 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
const (
	TagExtensionCP                 = 0x02
	TagExtensionCPIdentifier       = 0x03
	TagExtensionC2DeliverySystem   = 0x0d
	TagExtensionSupplementaryAudio = 0x6
	TagExtensionCIAncillaryData    = 0x14
)

// Extension represents an extension descriptor
// Chapter: 6.2.16 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type Extension struct {
	SupplementaryAudio *ExtensionSupplementaryAudio
	CIAncillaryData    *ExtensionCIAncillaryData
	CP                 *ExtensionCP
	CPIdentifier       *ExtensionCPIdentifier
	C2DeliverySystem   *ExtensionC2DeliverySystem
	Unknown            []byte
	Header             Header
	Tag                uint8
}

func newDescriptorExtension(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	d := &Extension{
		Header: h,
		Tag:    b,
	}
	dd = d

	switch d.Tag {
	case TagExtensionSupplementaryAudio:
		if d.SupplementaryAudio, err = newDescriptorExtensionSupplementaryAudio(i, offsetEnd); err != nil {
			err = fmt.Errorf("astits: parsing extension supplementary audio descriptor failed: %w", err)
			return
		}
	case TagExtensionCIAncillaryData:
		if d.CIAncillaryData, err = newDescriptorExtensionCIAncillaryData(i, offsetEnd); err != nil {
			err = fmt.Errorf("astits: parsing extension CI ancillary data descriptor failed: %w", err)
			return
		}
	case TagExtensionCP:
		if d.CP, err = newDescriptorExtensionCP(i, offsetEnd); err != nil {
			err = fmt.Errorf("astits: parsing extension CP descriptor failed: %w", err)
			return
		}
	case TagExtensionCPIdentifier:
		if d.CPIdentifier, err = newDescriptorExtensionCPIdentifier(i, offsetEnd); err != nil {
			err = fmt.Errorf("astits: parsing extension CP identifier descriptor failed: %w", err)
			return
		}
	case TagExtensionC2DeliverySystem:
		if d.C2DeliverySystem, err = newDescriptorExtensionC2DeliverySystem(i, offsetEnd); err != nil {
			err = fmt.Errorf("astits: parsing extension C2 delivery system descriptor failed: %w", err)
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

func (d *Extension) CalcLength() int {
	ret := 1 // tag

	switch d.Tag {
	case TagExtensionSupplementaryAudio:
		ret += d.SupplementaryAudio.CalcLength()
	case TagExtensionCIAncillaryData:
		ret += d.CIAncillaryData.CalcLength()
	case TagExtensionCP:
		ret += d.CP.CalcLength()
	case TagExtensionCPIdentifier:
		ret += d.CPIdentifier.CalcLength()
	case TagExtensionC2DeliverySystem:
		ret += d.C2DeliverySystem.CalcLength()
	default:
		ret += len(d.Unknown)
	}

	return ret
}

func (d *Extension) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, d.Tag)

	switch d.Tag {
	case TagExtensionSupplementaryAudio:
		dst = d.SupplementaryAudio.Append(dst)
	case TagExtensionCIAncillaryData:
		dst = d.CIAncillaryData.Append(dst)
	case TagExtensionCP:
		dst = d.CP.Append(dst)
	case TagExtensionCPIdentifier:
		dst = d.CPIdentifier.Append(dst)
	case TagExtensionC2DeliverySystem:
		dst = d.C2DeliverySystem.Append(dst)
	default:
		dst = append(dst, d.Unknown...)
	}

	return dst
}

// ExtensionSupplementaryAudio represents a supplementary audio extension descriptor
// Chapter: 6.4.10 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type ExtensionSupplementaryAudio struct {
	PrivateData             []byte
	LanguageCode            [3]byte
	EditorialClassification uint8
	HasLanguageCode         bool
	MixType                 bool
}

func newDescriptorExtensionSupplementaryAudio(i *bytesiter.Iterator, offsetEnd int) (d *ExtensionSupplementaryAudio, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	d = &ExtensionSupplementaryAudio{
		EditorialClassification: b >> 2 & 0x1f,
		HasLanguageCode:         b&0x1 > 0,
		MixType:                 b&0x80 > 0,
	}

	if d.HasLanguageCode {
		var bs []byte
		if bs, err = i.NextBytesNoCopy(3); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		copy(d.LanguageCode[:], bs)
	}

	if i.Offset() < offsetEnd {
		if d.PrivateData, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *ExtensionSupplementaryAudio) CalcLength() int {
	ret := 1
	ret += 3 * int(util.B2U(d.HasLanguageCode))
	ret += len(d.PrivateData)
	return ret
}

// Append appends the extension body only: tag and length are owned by the
// enclosing Extension descriptor.
func (d *ExtensionSupplementaryAudio) Append(dst []byte) []byte {
	dst = append(dst, util.B2U(d.MixType)<<7|d.EditorialClassification&0x1f<<2|1<<1|util.B2U(d.HasLanguageCode))

	if d.HasLanguageCode {
		dst = append(dst, d.LanguageCode[:]...)
	}

	return append(dst, d.PrivateData...)
}
