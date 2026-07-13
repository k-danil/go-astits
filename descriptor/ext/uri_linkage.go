package ext

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

type URILinkageType uint8

// uri_linkage_type values (EN 300 468 Table 151)
const (
	URILinkageTypeOnlineSDT URILinkageType = 0x00
	URILinkageTypeIPTVSDS   URILinkageType = 0x01
	URILinkageTypeMRS       URILinkageType = 0x02
)

var uriLinkageTypeNames = map[URILinkageType]string{
	URILinkageTypeOnlineSDT: "online_SDT",
	URILinkageTypeIPTVSDS:   "DVB_IPTV_SDS",
	URILinkageTypeMRS:       "material_resolution_server",
}

func (t URILinkageType) String() (s string) {
	var ok bool
	if s, ok = uriLinkageTypeNames[t]; !ok {
		s = fmt.Sprintf("0x%02x", uint8(t))
	}
	return
}

func (t URILinkageType) MarshalJSON() (b []byte, err error) {
	return json.Marshal(t.String())
}

func (t *URILinkageType) UnmarshalJSON(b []byte) (err error) {
	*t, err = util.UnmarshalEnum(b, uriLinkageTypeNames)
	return
}

// URILinkage represents a URI linkage extension descriptor: a resource
// reachable over IP. MinPollingInterval is present for URILinkageType 0 and 1.
// Chapter: 6.4.14 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type URILinkage struct {
	URI                []byte         `json:"uri_char"`
	PrivateData        []byte         `json:"private_data"`
	MinPollingInterval uint16         `json:"min_polling_interval"`
	URILinkageType     URILinkageType `json:"uri_linkage_type"`
}

func uriLinkageHasPollingInterval(uriLinkageType URILinkageType) bool {
	return uriLinkageType == URILinkageTypeOnlineSDT || uriLinkageType == URILinkageTypeIPTVSDS
}

func parseURILinkage(i *bytesiter.Iterator, offsetEnd int) (d *URILinkage, err error) {
	d = &URILinkage{}

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.URILinkageType = URILinkageType(b)
	if err = readLengthPrefixed(i, &d.URI); err != nil {
		return
	}
	if uriLinkageHasPollingInterval(d.URILinkageType) {
		var bs []byte
		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		d.MinPollingInterval = binary.BigEndian.Uint16(bs)
	}
	if i.Offset() < offsetEnd {
		if d.PrivateData, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *URILinkage) CalcLength() int {
	n := 2 + len(d.URI) + len(d.PrivateData)
	if uriLinkageHasPollingInterval(d.URILinkageType) {
		n += 2
	}
	return n
}

func (d *URILinkage) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.URILinkageType), uint8(len(d.URI)))
	dst = append(dst, d.URI...)
	if uriLinkageHasPollingInterval(d.URILinkageType) {
		dst = append(dst, byte(d.MinPollingInterval>>8), byte(d.MinPollingInterval))
	}
	return append(dst, d.PrivateData...)
}
