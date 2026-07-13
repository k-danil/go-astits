package descriptor

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

// AnnouncementSupport represents an announcement support descriptor: the
// announcement types a service supports and, for referenced ones, how to reach
// the announcement stream.
// Chapter: 6.2.3 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type AnnouncementSupport struct {
	Announcements    []AnnouncementSupportItem `json:"_announcements"`
	Header           Header                    `json:"_header"`
	SupportIndicator uint16                    `json:"announcement_support_indicator"`
}

// AnnouncementSupportItem is one announcement entry; the reference fields are
// present only when ReferenceType is 1, 2 or 3.
type AnnouncementSupportItem struct {
	OriginalNetworkID uint16                `json:"original_network_id"`
	TransportStreamID uint16                `json:"transport_stream_id"`
	ServiceID         uint16                `json:"service_id"`
	AnnouncementType  AnnouncementType      `json:"announcement_type"`
	ReferenceType     AnnouncementReference `json:"reference_type"`
	ComponentTag      uint8                 `json:"component_tag"`
}

type AnnouncementType uint8

// announcement_type values (EN 300 468 Table 19)
const (
	AnnouncementTypeEmergencyAlarm       AnnouncementType = 0x00
	AnnouncementTypeRoadTrafficFlash     AnnouncementType = 0x01
	AnnouncementTypePublicTransportFlash AnnouncementType = 0x02
	AnnouncementTypeWarningMessage       AnnouncementType = 0x03
	AnnouncementTypeNewsFlash            AnnouncementType = 0x04
	AnnouncementTypeWeatherFlash         AnnouncementType = 0x05
	AnnouncementTypeEventAnnouncement    AnnouncementType = 0x06
	AnnouncementTypePersonalCall         AnnouncementType = 0x07
)

var announcementTypeNames = map[AnnouncementType]string{
	AnnouncementTypeEmergencyAlarm:       "emergency_alarm",
	AnnouncementTypeRoadTrafficFlash:     "road_traffic_flash",
	AnnouncementTypePublicTransportFlash: "public_transport_flash",
	AnnouncementTypeWarningMessage:       "warning_message",
	AnnouncementTypeNewsFlash:            "news_flash",
	AnnouncementTypeWeatherFlash:         "weather_flash",
	AnnouncementTypeEventAnnouncement:    "event_announcement",
	AnnouncementTypePersonalCall:         "personal_call",
}

func (t AnnouncementType) String() (s string) {
	var ok bool
	if s, ok = announcementTypeNames[t]; !ok {
		s = fmt.Sprintf("0x%02x", uint8(t))
	}
	return
}

func (t AnnouncementType) MarshalJSON() (b []byte, err error) {
	return json.Marshal(t.String())
}

func (t *AnnouncementType) UnmarshalJSON(b []byte) (err error) {
	*t, err = util.UnmarshalEnum(b, announcementTypeNames)
	return
}

type AnnouncementReference uint8

// reference_type values (EN 300 468 Table 20)
const (
	AnnouncementReferenceUsualAudio       AnnouncementReference = 0x00
	AnnouncementReferenceSeparateAudio    AnnouncementReference = 0x01
	AnnouncementReferenceDifferentService AnnouncementReference = 0x02
	AnnouncementReferenceDifferentTS      AnnouncementReference = 0x03
)

var announcementReferenceNames = map[AnnouncementReference]string{
	AnnouncementReferenceUsualAudio:       "usual_audio_stream",
	AnnouncementReferenceSeparateAudio:    "separate_audio_stream",
	AnnouncementReferenceDifferentService: "different_service",
	AnnouncementReferenceDifferentTS:      "different_transport_stream",
}

func (t AnnouncementReference) String() (s string) {
	var ok bool
	if s, ok = announcementReferenceNames[t]; !ok {
		s = fmt.Sprintf("0x%02x", uint8(t))
	}
	return
}

func (t AnnouncementReference) MarshalJSON() (b []byte, err error) {
	return json.Marshal(t.String())
}

func (t *AnnouncementReference) UnmarshalJSON(b []byte) (err error) {
	*t, err = util.UnmarshalEnum(b, announcementReferenceNames)
	return
}

func announcementHasReference(referenceType AnnouncementReference) bool {
	return referenceType >= AnnouncementReferenceSeparateAudio && referenceType <= AnnouncementReferenceDifferentTS
}

func newDescriptorAnnouncementSupport(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &AnnouncementSupport{
		Header: h,
	}
	dd = d

	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d.SupportIndicator = binary.BigEndian.Uint16(bs)

	for i.Offset() < offsetEnd {
		var item AnnouncementSupportItem
		var b byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		item.AnnouncementType = AnnouncementType(b >> 4 & 0x0f)
		item.ReferenceType = AnnouncementReference(b & 0x07)

		if announcementHasReference(item.ReferenceType) {
			if bs, err = i.NextBytesNoCopy(7); err != nil || len(bs) < 7 {
				err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
				return
			}
			item.OriginalNetworkID = binary.BigEndian.Uint16(bs[0:2])
			item.TransportStreamID = binary.BigEndian.Uint16(bs[2:4])
			item.ServiceID = binary.BigEndian.Uint16(bs[4:6])
			item.ComponentTag = bs[6]
		}
		d.Announcements = append(d.Announcements, item)
	}
	return
}

func (d *AnnouncementSupport) CalcLength() (n int) {
	n = 2
	for _, item := range d.Announcements {
		n++
		if announcementHasReference(item.ReferenceType) {
			n += 7
		}
	}
	return
}

func (d *AnnouncementSupport) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, byte(d.SupportIndicator>>8), byte(d.SupportIndicator))
	for _, item := range d.Announcements {
		dst = append(dst, uint8(item.AnnouncementType)&0x0f<<4|0x08|uint8(item.ReferenceType)&0x07)
		if announcementHasReference(item.ReferenceType) {
			dst = append(dst,
				byte(item.OriginalNetworkID>>8), byte(item.OriginalNetworkID),
				byte(item.TransportStreamID>>8), byte(item.TransportStreamID),
				byte(item.ServiceID>>8), byte(item.ServiceID),
				item.ComponentTag)
		}
	}
	return dst
}
