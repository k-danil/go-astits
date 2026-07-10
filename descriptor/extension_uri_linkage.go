package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// ExtensionURILinkage represents a URI linkage extension descriptor: a resource
// reachable over IP. MinPollingInterval is present for URILinkageType 0 and 1.
// Chapter: 6.4.14 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type ExtensionURILinkage struct {
	URI                []byte
	PrivateData        []byte
	MinPollingInterval uint16
	URILinkageType     uint8
}

func uriLinkageHasPollingInterval(uriLinkageType uint8) bool {
	return uriLinkageType == 0x00 || uriLinkageType == 0x01
}

func newDescriptorExtensionURILinkage(i *bytesiter.Iterator, offsetEnd int) (d *ExtensionURILinkage, err error) {
	d = &ExtensionURILinkage{}

	if d.URILinkageType, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
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

func (d *ExtensionURILinkage) CalcLength() int {
	n := 2 + len(d.URI) + len(d.PrivateData)
	if uriLinkageHasPollingInterval(d.URILinkageType) {
		n += 2
	}
	return n
}

func (d *ExtensionURILinkage) Append(dst []byte) []byte {
	dst = append(dst, d.URILinkageType, uint8(len(d.URI)))
	dst = append(dst, d.URI...)
	if uriLinkageHasPollingInterval(d.URILinkageType) {
		dst = append(dst, byte(d.MinPollingInterval>>8), byte(d.MinPollingInterval))
	}
	return append(dst, d.PrivateData...)
}
