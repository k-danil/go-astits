package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// AnnouncementSupport represents an announcement support descriptor: the
// announcement types a service supports and, for referenced ones, how to reach
// the announcement stream.
// Chapter: 6.2.3 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type AnnouncementSupport struct {
	Announcements    []AnnouncementSupportItem
	Header           Header
	SupportIndicator uint16
}

// AnnouncementSupportItem is one announcement entry; the reference fields are
// present only when ReferenceType is 1, 2 or 3.
type AnnouncementSupportItem struct {
	OriginalNetworkID uint16
	TransportStreamID uint16
	ServiceID         uint16
	AnnouncementType  uint8
	ReferenceType     uint8
	ComponentTag      uint8
}

func announcementHasReference(referenceType uint8) bool {
	return referenceType >= 0x01 && referenceType <= 0x03
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
		item.AnnouncementType = b >> 4 & 0x0f
		item.ReferenceType = b & 0x07

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
		dst = append(dst, item.AnnouncementType&0x0f<<4|0x08|item.ReferenceType&0x07)
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
