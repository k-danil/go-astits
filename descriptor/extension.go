package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

// Descriptor extension tags
// Chapter: 6.3 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
const (
	TagExtensionImageIcon              = 0x00
	TagExtensionCP                     = 0x02
	TagExtensionCPIdentifier           = 0x03
	TagExtensionT2DeliverySystem       = 0x04
	TagExtensionSHDeliverySystem       = 0x05
	TagExtensionSupplementaryAudio     = 0x6
	TagExtensionNetworkChangeNotify    = 0x07
	TagExtensionMessage                = 0x08
	TagExtensionTargetRegion           = 0x09
	TagExtensionTargetRegionName       = 0x0a
	TagExtensionServiceRelocated       = 0x0b
	TagExtensionC2DeliverySystem       = 0x0d
	TagExtensionDTSHD                  = 0x0e
	TagExtensionDTSNeural              = 0x0f
	TagExtensionVideoDepthRange        = 0x10
	TagExtensionT2MI                   = 0x11
	TagExtensionURILinkage             = 0x13
	TagExtensionCIAncillaryData        = 0x14
	TagExtensionAC4                    = 0x15
	TagExtensionC2BundleDeliverySystem = 0x16
)

// Extension represents an extension descriptor
// Chapter: 6.2.16 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type Extension struct {
	SupplementaryAudio     *ExtensionSupplementaryAudio
	CIAncillaryData        *ExtensionCIAncillaryData
	CP                     *ExtensionCP
	CPIdentifier           *ExtensionCPIdentifier
	C2DeliverySystem       *ExtensionC2DeliverySystem
	C2BundleDeliverySystem *ExtensionC2BundleDeliverySystem
	SHDeliverySystem       *ExtensionSHDeliverySystem
	T2DeliverySystem       *ExtensionT2DeliverySystem
	Message                *ExtensionMessage
	ServiceRelocated       *ExtensionServiceRelocated
	ImageIcon              *ExtensionImageIcon
	NetworkChangeNotify    *ExtensionNetworkChangeNotify
	TargetRegion           *ExtensionTargetRegion
	TargetRegionName       *ExtensionTargetRegionName
	T2MI                   *ExtensionT2MI
	URILinkage             *ExtensionURILinkage
	VideoDepthRange        *ExtensionVideoDepthRange
	DTSHD                  *ExtensionDTSHD
	DTSNeural              *ExtensionDTSNeural
	AC4                    *ExtensionAC4
	Unknown                []byte
	Header                 Header
	Tag                    uint8
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
	case TagExtensionC2BundleDeliverySystem:
		if d.C2BundleDeliverySystem, err = newDescriptorExtensionC2BundleDeliverySystem(i, offsetEnd); err != nil {
			err = fmt.Errorf("astits: parsing extension C2 bundle delivery system descriptor failed: %w", err)
			return
		}
	case TagExtensionSHDeliverySystem:
		if d.SHDeliverySystem, err = newDescriptorExtensionSHDeliverySystem(i, offsetEnd); err != nil {
			err = fmt.Errorf("astits: parsing extension SH delivery system descriptor failed: %w", err)
			return
		}
	case TagExtensionMessage:
		if d.Message, err = newDescriptorExtensionMessage(i, offsetEnd); err != nil {
			err = fmt.Errorf("astits: parsing extension message descriptor failed: %w", err)
			return
		}
	case TagExtensionServiceRelocated:
		if d.ServiceRelocated, err = newDescriptorExtensionServiceRelocated(i, offsetEnd); err != nil {
			err = fmt.Errorf("astits: parsing extension service relocated descriptor failed: %w", err)
			return
		}
	case TagExtensionT2DeliverySystem:
		if d.T2DeliverySystem, err = newDescriptorExtensionT2DeliverySystem(i, offsetEnd); err != nil {
			err = fmt.Errorf("astits: parsing extension T2 delivery system descriptor failed: %w", err)
			return
		}
	case TagExtensionImageIcon:
		if d.ImageIcon, err = newDescriptorExtensionImageIcon(i, offsetEnd); err != nil {
			err = fmt.Errorf("astits: parsing extension image icon descriptor failed: %w", err)
			return
		}
	case TagExtensionNetworkChangeNotify:
		if d.NetworkChangeNotify, err = newDescriptorExtensionNetworkChangeNotify(i, offsetEnd); err != nil {
			err = fmt.Errorf("astits: parsing extension network change notify descriptor failed: %w", err)
			return
		}
	case TagExtensionTargetRegion:
		if d.TargetRegion, err = newDescriptorExtensionTargetRegion(i, offsetEnd); err != nil {
			err = fmt.Errorf("astits: parsing extension target region descriptor failed: %w", err)
			return
		}
	case TagExtensionTargetRegionName:
		if d.TargetRegionName, err = newDescriptorExtensionTargetRegionName(i, offsetEnd); err != nil {
			err = fmt.Errorf("astits: parsing extension target region name descriptor failed: %w", err)
			return
		}
	case TagExtensionT2MI:
		if d.T2MI, err = newDescriptorExtensionT2MI(i, offsetEnd); err != nil {
			err = fmt.Errorf("astits: parsing extension T2-MI descriptor failed: %w", err)
			return
		}
	case TagExtensionURILinkage:
		if d.URILinkage, err = newDescriptorExtensionURILinkage(i, offsetEnd); err != nil {
			err = fmt.Errorf("astits: parsing extension URI linkage descriptor failed: %w", err)
			return
		}
	case TagExtensionVideoDepthRange:
		if d.VideoDepthRange, err = newDescriptorExtensionVideoDepthRange(i, offsetEnd); err != nil {
			err = fmt.Errorf("astits: parsing extension video depth range descriptor failed: %w", err)
			return
		}
	case TagExtensionDTSHD:
		if d.DTSHD, err = newDescriptorExtensionDTSHD(i, offsetEnd); err != nil {
			err = fmt.Errorf("astits: parsing extension DTS-HD descriptor failed: %w", err)
			return
		}
	case TagExtensionDTSNeural:
		if d.DTSNeural, err = newDescriptorExtensionDTSNeural(i, offsetEnd); err != nil {
			err = fmt.Errorf("astits: parsing extension DTS Neural descriptor failed: %w", err)
			return
		}
	case TagExtensionAC4:
		if d.AC4, err = newDescriptorExtensionAC4(i, offsetEnd); err != nil {
			err = fmt.Errorf("astits: parsing extension AC-4 descriptor failed: %w", err)
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
	case TagExtensionC2BundleDeliverySystem:
		ret += d.C2BundleDeliverySystem.CalcLength()
	case TagExtensionSHDeliverySystem:
		ret += d.SHDeliverySystem.CalcLength()
	case TagExtensionMessage:
		ret += d.Message.CalcLength()
	case TagExtensionServiceRelocated:
		ret += d.ServiceRelocated.CalcLength()
	case TagExtensionT2DeliverySystem:
		ret += d.T2DeliverySystem.CalcLength()
	case TagExtensionImageIcon:
		ret += d.ImageIcon.CalcLength()
	case TagExtensionNetworkChangeNotify:
		ret += d.NetworkChangeNotify.CalcLength()
	case TagExtensionTargetRegion:
		ret += d.TargetRegion.CalcLength()
	case TagExtensionTargetRegionName:
		ret += d.TargetRegionName.CalcLength()
	case TagExtensionT2MI:
		ret += d.T2MI.CalcLength()
	case TagExtensionURILinkage:
		ret += d.URILinkage.CalcLength()
	case TagExtensionVideoDepthRange:
		ret += d.VideoDepthRange.CalcLength()
	case TagExtensionDTSHD:
		ret += d.DTSHD.CalcLength()
	case TagExtensionDTSNeural:
		ret += d.DTSNeural.CalcLength()
	case TagExtensionAC4:
		ret += d.AC4.CalcLength()
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
	case TagExtensionC2BundleDeliverySystem:
		dst = d.C2BundleDeliverySystem.Append(dst)
	case TagExtensionSHDeliverySystem:
		dst = d.SHDeliverySystem.Append(dst)
	case TagExtensionMessage:
		dst = d.Message.Append(dst)
	case TagExtensionServiceRelocated:
		dst = d.ServiceRelocated.Append(dst)
	case TagExtensionT2DeliverySystem:
		dst = d.T2DeliverySystem.Append(dst)
	case TagExtensionImageIcon:
		dst = d.ImageIcon.Append(dst)
	case TagExtensionNetworkChangeNotify:
		dst = d.NetworkChangeNotify.Append(dst)
	case TagExtensionTargetRegion:
		dst = d.TargetRegion.Append(dst)
	case TagExtensionTargetRegionName:
		dst = d.TargetRegionName.Append(dst)
	case TagExtensionT2MI:
		dst = d.T2MI.Append(dst)
	case TagExtensionURILinkage:
		dst = d.URILinkage.Append(dst)
	case TagExtensionVideoDepthRange:
		dst = d.VideoDepthRange.Append(dst)
	case TagExtensionDTSHD:
		dst = d.DTSHD.Append(dst)
	case TagExtensionDTSNeural:
		dst = d.DTSNeural.Append(dst)
	case TagExtensionAC4:
		dst = d.AC4.Append(dst)
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
