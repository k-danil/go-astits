package ext

import (
	"encoding/json"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

type ImageIconTransportMode uint8

// icon_transport_mode values (EN 300 468 Table 137)
const (
	ImageIconTransportModeInline ImageIconTransportMode = 0x00
	ImageIconTransportModeURL    ImageIconTransportMode = 0x01
)

var imageIconTransportModeNames = map[ImageIconTransportMode]string{
	ImageIconTransportModeInline: "icon_data_bytes",
	ImageIconTransportModeURL:    "URL",
}

func (t ImageIconTransportMode) String() (s string) {
	var ok bool
	if s, ok = imageIconTransportModeNames[t]; !ok {
		s = fmt.Sprintf("0x%02x", uint8(t))
	}
	return
}

func (t ImageIconTransportMode) MarshalJSON() (b []byte, err error) {
	return json.Marshal(t.String())
}

func (t *ImageIconTransportMode) UnmarshalJSON(b []byte) (err error) {
	*t, err = util.UnmarshalEnum(b, imageIconTransportModeNames)
	return
}

// ImageIcon represents an image icon extension descriptor: inline icon
// data or a URL to an icon. The header/position/type fields are carried only in
// the first descriptor of a set (DescriptorNumber == 0); later ones carry a
// further chunk of icon data. IconData holds either the icon bytes
// (IconTransportMode 0) or the URL bytes (IconTransportMode 1).
// Chapter: 6.4.6 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type ImageIcon struct {
	IconType             []byte                 `json:"icon_type"`
	IconData             []byte                 `json:"icon_data_byte"`
	IconHorizontalOrigin uint16                 `json:"icon_horizontal_origin"`
	IconVerticalOrigin   uint16                 `json:"icon_vertical_origin"`
	DescriptorNumber     uint8                  `json:"descriptor_number"`
	LastDescriptorNumber uint8                  `json:"last_descriptor_number"`
	IconID               uint8                  `json:"icon_id"`
	IconTransportMode    ImageIconTransportMode `json:"icon_transport_mode"`
	CoordinateSystem     uint8                  `json:"coordinate_system"`
	PositionFlag         bool                   `json:"position_flag"`
}

func parseImageIcon(i *bytesiter.Iterator, _ int) (d *ImageIcon, err error) {
	d = &ImageIcon{}

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
	d.IconTransportMode = ImageIconTransportMode(b >> 6 & 0x03)
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
	if d.IconTransportMode == ImageIconTransportModeInline || d.IconTransportMode == ImageIconTransportModeURL {
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

func (d *ImageIcon) CalcLength() (n int) {
	n = 2
	if d.DescriptorNumber != 0 {
		return n + 1 + len(d.IconData)
	}
	n++
	if d.PositionFlag {
		n += 3
	}
	n += 1 + len(d.IconType)
	if d.IconTransportMode == ImageIconTransportModeInline || d.IconTransportMode == ImageIconTransportModeURL {
		n += 1 + len(d.IconData)
	}
	return
}

func (d *ImageIcon) Append(dst []byte) []byte {
	dst = append(dst, d.DescriptorNumber&0x0f<<4|d.LastDescriptorNumber&0x0f, 0xf8|d.IconID&0x07)

	if d.DescriptorNumber != 0 {
		dst = append(dst, uint8(len(d.IconData)))
		return append(dst, d.IconData...)
	}

	b := uint8(d.IconTransportMode)&0x03<<6 | 0x1f
	if d.PositionFlag {
		b = uint8(d.IconTransportMode)&0x03<<6 | 0x20 | d.CoordinateSystem&0x07<<2 | 0x03
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
	if d.IconTransportMode == ImageIconTransportModeInline || d.IconTransportMode == ImageIconTransportModeURL {
		dst = append(dst, uint8(len(d.IconData)))
		dst = append(dst, d.IconData...)
	}
	return dst
}
