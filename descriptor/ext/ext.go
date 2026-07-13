// Package ext holds the DVB extension_descriptor sub-descriptors (EN 300 468
// §6.3): the extension_descriptor (main tag 0x7f) selects one of these by a
// second extension_descriptor_tag byte. Each Body is a payload only; the outer
// tag and length are owned by the enclosing descriptor.Extension.
package ext

import (
	"encoding/json"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

type Tag uint8

// Body is one extension_descriptor sub-descriptor. Tag reports its
// extension_descriptor_tag; CalcLength and Append cover the payload only.
type Body interface {
	Tag() Tag
	CalcLength() int
	Append(dst []byte) []byte
}

// Extension descriptor tags
// Chapter: 6.3 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
const (
	TagImageIcon              Tag = 0x00
	TagCP                     Tag = 0x02
	TagCPIdentifier           Tag = 0x03
	TagT2DeliverySystem       Tag = 0x04
	TagSHDeliverySystem       Tag = 0x05
	TagSupplementaryAudio     Tag = 0x6
	TagNetworkChangeNotify    Tag = 0x07
	TagMessage                Tag = 0x08
	TagTargetRegion           Tag = 0x09
	TagTargetRegionName       Tag = 0x0a
	TagServiceRelocated       Tag = 0x0b
	TagC2DeliverySystem       Tag = 0x0d
	TagDTSHD                  Tag = 0x0e
	TagDTSNeural              Tag = 0x0f
	TagVideoDepthRange        Tag = 0x10
	TagT2MI                   Tag = 0x11
	TagURILinkage             Tag = 0x13
	TagCIAncillaryData        Tag = 0x14
	TagAC4                    Tag = 0x15
	TagC2BundleDeliverySystem Tag = 0x16
)

var tagNames = map[Tag]string{
	TagImageIcon:              "image_icon_descriptor",
	TagCP:                     "CP_descriptor",
	TagCPIdentifier:           "CP_identifier_descriptor",
	TagT2DeliverySystem:       "T2_delivery_system_descriptor",
	TagSHDeliverySystem:       "SH_delivery_system_descriptor",
	TagSupplementaryAudio:     "supplementary_audio_descriptor",
	TagNetworkChangeNotify:    "network_change_notify_descriptor",
	TagMessage:                "message_descriptor",
	TagTargetRegion:           "target_region_descriptor",
	TagTargetRegionName:       "target_region_name_descriptor",
	TagServiceRelocated:       "service_relocated_descriptor",
	TagC2DeliverySystem:       "C2_delivery_system_descriptor",
	TagDTSHD:                  "DTS-HD_audio_stream_descriptor",
	TagDTSNeural:              "DTS_Neural_descriptor",
	TagVideoDepthRange:        "video_depth_range_descriptor",
	TagT2MI:                   "T2MI_descriptor",
	TagURILinkage:             "URI_linkage_descriptor",
	TagCIAncillaryData:        "CI_ancillary_data_descriptor",
	TagAC4:                    "AC-4_descriptor",
	TagC2BundleDeliverySystem: "C2_bundle_delivery_system_descriptor",
}

func (t Tag) String() (s string) {
	var ok bool
	if s, ok = tagNames[t]; !ok {
		s = fmt.Sprintf("0x%02x", uint8(t))
	}
	return
}

func (t Tag) MarshalJSON() (b []byte, err error) {
	return json.Marshal(t.String())
}

func (t *Tag) UnmarshalJSON(b []byte) (err error) {
	*t, err = util.UnmarshalEnum(b, tagNames)
	return
}

func (*SupplementaryAudio) Tag() Tag     { return TagSupplementaryAudio }
func (*CIAncillaryData) Tag() Tag        { return TagCIAncillaryData }
func (*CP) Tag() Tag                     { return TagCP }
func (*CPIdentifier) Tag() Tag           { return TagCPIdentifier }
func (*C2DeliverySystem) Tag() Tag       { return TagC2DeliverySystem }
func (*C2BundleDeliverySystem) Tag() Tag { return TagC2BundleDeliverySystem }
func (*SHDeliverySystem) Tag() Tag       { return TagSHDeliverySystem }
func (*Message) Tag() Tag                { return TagMessage }
func (*ServiceRelocated) Tag() Tag       { return TagServiceRelocated }
func (*T2DeliverySystem) Tag() Tag       { return TagT2DeliverySystem }
func (*ImageIcon) Tag() Tag              { return TagImageIcon }
func (*NetworkChangeNotify) Tag() Tag    { return TagNetworkChangeNotify }
func (*TargetRegion) Tag() Tag           { return TagTargetRegion }
func (*TargetRegionName) Tag() Tag       { return TagTargetRegionName }
func (*T2MI) Tag() Tag                   { return TagT2MI }
func (*URILinkage) Tag() Tag             { return TagURILinkage }
func (*VideoDepthRange) Tag() Tag        { return TagVideoDepthRange }
func (*DTSHD) Tag() Tag                  { return TagDTSHD }
func (*DTSNeural) Tag() Tag              { return TagDTSNeural }
func (*AC4) Tag() Tag                    { return TagAC4 }

// Unknown is an extension_descriptor sub-descriptor whose tag is not typed; it
// carries the raw payload verbatim.
type Unknown struct {
	Data   []byte `json:"_data"`
	ExtTag Tag    `json:"extension_descriptor_tag"`
}

func (u *Unknown) Tag() Tag                 { return u.ExtTag }
func (u *Unknown) CalcLength() int          { return len(u.Data) }
func (u *Unknown) Append(dst []byte) []byte { return append(dst, u.Data...) }

// Parse reads the sub-descriptor body for extTag; unrecognised tags become an
// Unknown holding the remaining bytes up to offsetEnd.
func Parse(i *bytesiter.Iterator, extTag Tag, offsetEnd int) (b Body, err error) {
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
