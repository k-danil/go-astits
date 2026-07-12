// Package ext holds the DVB extension_descriptor sub-descriptors (EN 300 468
// §6.3): the extension_descriptor (main tag 0x7f) selects one of these by a
// second extension_descriptor_tag byte. Each Body is a payload only; the outer
// tag and length are owned by the enclosing descriptor.Extension.
package ext

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// Body is one extension_descriptor sub-descriptor. Tag reports its
// extension_descriptor_tag; CalcLength and Append cover the payload only.
type Body interface {
	Tag() uint8
	CalcLength() int
	Append(dst []byte) []byte
}

// Extension descriptor tags
// Chapter: 6.3 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
const (
	TagImageIcon              = 0x00
	TagCP                     = 0x02
	TagCPIdentifier           = 0x03
	TagT2DeliverySystem       = 0x04
	TagSHDeliverySystem       = 0x05
	TagSupplementaryAudio     = 0x6
	TagNetworkChangeNotify    = 0x07
	TagMessage                = 0x08
	TagTargetRegion           = 0x09
	TagTargetRegionName       = 0x0a
	TagServiceRelocated       = 0x0b
	TagC2DeliverySystem       = 0x0d
	TagDTSHD                  = 0x0e
	TagDTSNeural              = 0x0f
	TagVideoDepthRange        = 0x10
	TagT2MI                   = 0x11
	TagURILinkage             = 0x13
	TagCIAncillaryData        = 0x14
	TagAC4                    = 0x15
	TagC2BundleDeliverySystem = 0x16
)

func (*SupplementaryAudio) Tag() uint8     { return TagSupplementaryAudio }
func (*CIAncillaryData) Tag() uint8        { return TagCIAncillaryData }
func (*CP) Tag() uint8                     { return TagCP }
func (*CPIdentifier) Tag() uint8           { return TagCPIdentifier }
func (*C2DeliverySystem) Tag() uint8       { return TagC2DeliverySystem }
func (*C2BundleDeliverySystem) Tag() uint8 { return TagC2BundleDeliverySystem }
func (*SHDeliverySystem) Tag() uint8       { return TagSHDeliverySystem }
func (*Message) Tag() uint8                { return TagMessage }
func (*ServiceRelocated) Tag() uint8       { return TagServiceRelocated }
func (*T2DeliverySystem) Tag() uint8       { return TagT2DeliverySystem }
func (*ImageIcon) Tag() uint8              { return TagImageIcon }
func (*NetworkChangeNotify) Tag() uint8    { return TagNetworkChangeNotify }
func (*TargetRegion) Tag() uint8           { return TagTargetRegion }
func (*TargetRegionName) Tag() uint8       { return TagTargetRegionName }
func (*T2MI) Tag() uint8                   { return TagT2MI }
func (*URILinkage) Tag() uint8             { return TagURILinkage }
func (*VideoDepthRange) Tag() uint8        { return TagVideoDepthRange }
func (*DTSHD) Tag() uint8                  { return TagDTSHD }
func (*DTSNeural) Tag() uint8              { return TagDTSNeural }
func (*AC4) Tag() uint8                    { return TagAC4 }

// Unknown is an extension_descriptor sub-descriptor whose tag is not typed; it
// carries the raw payload verbatim.
type Unknown struct {
	Data   []byte
	ExtTag uint8
}

func (u *Unknown) Tag() uint8              { return u.ExtTag }
func (u *Unknown) CalcLength() int         { return len(u.Data) }
func (u *Unknown) Append(dst []byte) []byte { return append(dst, u.Data...) }

// Parse reads the sub-descriptor body for extTag; unrecognised tags become an
// Unknown holding the remaining bytes up to offsetEnd.
func Parse(i *bytesiter.Iterator, extTag uint8, offsetEnd int) (b Body, err error) {
	switch extTag {
	case TagSupplementaryAudio:
		return parseSupplementaryAudio(i, offsetEnd)
	case TagCIAncillaryData:
		return parseCIAncillaryData(i, offsetEnd)
	case TagCP:
		return parseCP(i, offsetEnd)
	case TagCPIdentifier:
		return parseCPIdentifier(i, offsetEnd)
	case TagC2DeliverySystem:
		return parseC2DeliverySystem(i, offsetEnd)
	case TagC2BundleDeliverySystem:
		return parseC2BundleDeliverySystem(i, offsetEnd)
	case TagSHDeliverySystem:
		return parseSHDeliverySystem(i, offsetEnd)
	case TagMessage:
		return parseMessage(i, offsetEnd)
	case TagServiceRelocated:
		return parseServiceRelocated(i, offsetEnd)
	case TagT2DeliverySystem:
		return parseT2DeliverySystem(i, offsetEnd)
	case TagImageIcon:
		return parseImageIcon(i, offsetEnd)
	case TagNetworkChangeNotify:
		return parseNetworkChangeNotify(i, offsetEnd)
	case TagTargetRegion:
		return parseTargetRegion(i, offsetEnd)
	case TagTargetRegionName:
		return parseTargetRegionName(i, offsetEnd)
	case TagT2MI:
		return parseT2MI(i, offsetEnd)
	case TagURILinkage:
		return parseURILinkage(i, offsetEnd)
	case TagVideoDepthRange:
		return parseVideoDepthRange(i, offsetEnd)
	case TagDTSHD:
		return parseDTSHD(i, offsetEnd)
	case TagDTSNeural:
		return parseDTSNeural(i, offsetEnd)
	case TagAC4:
		return parseAC4(i, offsetEnd)
	default:
		var data []byte
		if data, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		return &Unknown{ExtTag: extTag, Data: data}, nil
	}
}
