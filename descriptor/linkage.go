package descriptor

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

type LinkageType uint8

// linkage_type values (EN 300 468 Table 58)
const (
	LinkageTypeInformationService            LinkageType = 0x01
	LinkageTypeEPGService                    LinkageType = 0x02
	LinkageTypeCAReplacementService          LinkageType = 0x03
	LinkageTypeTSContainingCompleteNetworkSI LinkageType = 0x04
	LinkageTypeServiceReplacementService     LinkageType = 0x05
	LinkageTypeDataBroadcastService          LinkageType = 0x06
	LinkageTypeRCSMap                        LinkageType = 0x07
	LinkageTypeMobileHandOver                LinkageType = 0x08
	LinkageTypeSystemSoftwareUpdateService   LinkageType = 0x09
	LinkageTypeTSContainingSSUBATOrNIT       LinkageType = 0x0a
	LinkageTypeIPMACNotificationService      LinkageType = 0x0b
	LinkageTypeTSContainingINTBATOrNIT       LinkageType = 0x0c
	LinkageTypeEventLinkage                  LinkageType = 0x0d
)

var linkageTypeNames = map[LinkageType]string{
	LinkageTypeInformationService:            "information_service",
	LinkageTypeEPGService:                    "EPG_service",
	LinkageTypeCAReplacementService:          "CA_replacement_service",
	LinkageTypeTSContainingCompleteNetworkSI: "TS_containing_complete_network_bouquet_SI",
	LinkageTypeServiceReplacementService:     "service_replacement_service",
	LinkageTypeDataBroadcastService:          "data_broadcast_service",
	LinkageTypeRCSMap:                        "RCS_Map",
	LinkageTypeMobileHandOver:                "mobile_hand_over",
	LinkageTypeSystemSoftwareUpdateService:   "system_software_update_service",
	LinkageTypeTSContainingSSUBATOrNIT:       "TS_containing_SSU_BAT_or_NIT",
	LinkageTypeIPMACNotificationService:      "IP_MAC_notification_service",
	LinkageTypeTSContainingINTBATOrNIT:       "TS_containing_INT_BAT_or_NIT",
	LinkageTypeEventLinkage:                  "event_linkage",
}

func (t LinkageType) String() (s string) {
	var ok bool
	if s, ok = linkageTypeNames[t]; !ok {
		s = fmt.Sprintf("0x%02x", uint8(t))
	}
	return
}

func (t LinkageType) MarshalJSON() (b []byte, err error) {
	return json.Marshal(t.String())
}

func (t *LinkageType) UnmarshalJSON(b []byte) (err error) {
	*t, err = util.UnmarshalEnum(b, linkageTypeNames)
	return
}

// Linkage represents a linkage descriptor: the service that provides extra
// information about the entity the descriptor sits in. The type-specific
// linkage info (mobile hand-over / event / extended-event, keyed by
// LinkageType) plus the trailing private data are kept raw in Data.
// Chapter: 6.2.19 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type Linkage struct {
	Data              []byte      `json:"private_data_byte"`
	Header            Header      `json:"_header"`
	TransportStreamID uint16      `json:"transport_stream_id"`
	OriginalNetworkID uint16      `json:"original_network_id"`
	ServiceID         uint16      `json:"service_id"`
	LinkageType       LinkageType `json:"linkage_type"`
}

func newDescriptorLinkage(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &Linkage{
		Header: h,
	}
	dd = d

	var bs []byte
	if bs, err = i.NextBytesNoCopy(7); err != nil || len(bs) < 7 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d.TransportStreamID = binary.BigEndian.Uint16(bs[0:2])
	d.OriginalNetworkID = binary.BigEndian.Uint16(bs[2:4])
	d.ServiceID = binary.BigEndian.Uint16(bs[4:6])
	d.LinkageType = LinkageType(bs[6])

	if d.Data, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *Linkage) CalcLength() int {
	return 7 + len(d.Data)
}

func (d *Linkage) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst,
		byte(d.TransportStreamID>>8), byte(d.TransportStreamID),
		byte(d.OriginalNetworkID>>8), byte(d.OriginalNetworkID),
		byte(d.ServiceID>>8), byte(d.ServiceID),
		uint8(d.LinkageType))
	return append(dst, d.Data...)
}
