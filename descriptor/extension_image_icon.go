package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// ExtensionImageIcon represents an image icon extension descriptor: inline icon
// data or a URL to an icon. The header/position/type fields are carried only in
// the first descriptor of a set (DescriptorNumber == 0); later ones carry a
// further chunk of icon data. IconData holds either the icon bytes
// (IconTransportMode 0) or the URL bytes (IconTransportMode 1).
// Chapter: 6.4.6 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type ExtensionImageIcon struct {
	IconType             []byte
	IconData             []byte
	IconHorizontalOrigin uint16
	IconVerticalOrigin   uint16
	DescriptorNumber     uint8
	LastDescriptorNumber uint8
	IconID               uint8
	IconTransportMode    uint8
	CoordinateSystem     uint8
	PositionFlag         bool
}

func newDescriptorExtensionImageIcon(i *bytesiter.Iterator, _ int) (d *ExtensionImageIcon, err error) {
	d = &ExtensionImageIcon{}

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.DescriptorNumber = b >> 4 & 0x0f
	d.LastDescriptorNumber = b & 0x0f

	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.IconID = b & 0x07

	if d.DescriptorNumber != 0 {
		return d, readLengthPrefixed(i, &d.IconData)
	}

	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.IconTransportMode = b >> 6 & 0x03
	d.PositionFlag = b&0x20 > 0

	if d.PositionFlag {
		d.CoordinateSystem = b >> 2 & 0x07
		var bs []byte
		if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		d.IconHorizontalOrigin = uint16(bs[0])<<4 | uint16(bs[1])>>4
		d.IconVerticalOrigin = uint16(bs[1]&0x0f)<<8 | uint16(bs[2])
	}

	if err = readLengthPrefixed(i, &d.IconType); err != nil {
		return
	}
	if d.IconTransportMode == 0x00 || d.IconTransportMode == 0x01 {
		if err = readLengthPrefixed(i, &d.IconData); err != nil {
			return
		}
	}
	return
}

func readLengthPrefixed(i *bytesiter.Iterator, dst *[]byte) (err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	if *dst, err = i.NextBytes(int(b)); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *ExtensionImageIcon) CalcLength() (n int) {
	n = 2
	if d.DescriptorNumber != 0 {
		return n + 1 + len(d.IconData)
	}
	n++
	if d.PositionFlag {
		n += 3
	}
	n += 1 + len(d.IconType)
	if d.IconTransportMode == 0x00 || d.IconTransportMode == 0x01 {
		n += 1 + len(d.IconData)
	}
	return
}

func (d *ExtensionImageIcon) Append(dst []byte) []byte {
	dst = append(dst, d.DescriptorNumber&0x0f<<4|d.LastDescriptorNumber&0x0f, 0xf8|d.IconID&0x07)

	if d.DescriptorNumber != 0 {
		dst = append(dst, uint8(len(d.IconData)))
		return append(dst, d.IconData...)
	}

	b := d.IconTransportMode&0x03<<6 | 0x1f
	if d.PositionFlag {
		b = d.IconTransportMode&0x03<<6 | 0x20 | d.CoordinateSystem&0x07<<2 | 0x03
	}
	dst = append(dst, b)
	if d.PositionFlag {
		dst = append(dst,
			byte(d.IconHorizontalOrigin>>4),
			byte(d.IconHorizontalOrigin&0x0f)<<4|byte(d.IconVerticalOrigin>>8&0x0f),
			byte(d.IconVerticalOrigin))
	}

	dst = append(dst, uint8(len(d.IconType)))
	dst = append(dst, d.IconType...)
	if d.IconTransportMode == 0x00 || d.IconTransportMode == 0x01 {
		dst = append(dst, uint8(len(d.IconData)))
		dst = append(dst, d.IconData...)
	}
	return dst
}
