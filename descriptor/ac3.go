package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

// AC3 represents an AC3 descriptor
// Chapter: Annex D | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type AC3 struct {
	Header           Header `json:"_header"`
	AdditionalInfo   []byte `json:"additional_info_byte"`
	ASVC             uint8  `json:"asvc"`
	BSID             uint8  `json:"bsid"`
	ComponentType    uint8  `json:"component_type"`
	HasASVC          bool   `json:"asvc_flag"`
	HasBSID          bool   `json:"bsid_flag"`
	HasComponentType bool   `json:"component_type_flag"`
	HasMainID        bool   `json:"mainid_flag"`
	MainID           uint8  `json:"mainid"`
}

func newDescriptorAC3(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	d := &AC3{
		Header:           h,
		HasASVC:          uint8(b&0x10) > 0,
		HasBSID:          uint8(b&0x40) > 0,
		HasComponentType: uint8(b&0x80) > 0,
		HasMainID:        uint8(b&0x20) > 0,
	}
	dd = d

	if d.HasComponentType {
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.ComponentType = b
	}

	if d.HasBSID {
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.BSID = b
	}

	if d.HasMainID {
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.MainID = b
	}

	if d.HasASVC {
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.ASVC = b
	}

	if i.Offset() < offsetEnd {
		if d.AdditionalInfo, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *AC3) CalcLength() int {
	ret := 1 // flags
	ret += int(util.B2U(d.HasComponentType))
	ret += int(util.B2U(d.HasBSID))
	ret += int(util.B2U(d.HasMainID))
	ret += int(util.B2U(d.HasASVC))
	ret += len(d.AdditionalInfo)

	return ret
}

func (d *AC3) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, util.B2U(d.HasComponentType)<<7|util.B2U(d.HasBSID)<<6|util.B2U(d.HasMainID)<<5|util.B2U(d.HasASVC)<<4|0xf)

	if d.HasComponentType {
		dst = append(dst, d.ComponentType)
	}
	if d.HasBSID {
		dst = append(dst, d.BSID)
	}
	if d.HasMainID {
		dst = append(dst, d.MainID)
	}
	if d.HasASVC {
		dst = append(dst, d.ASVC)
	}
	return append(dst, d.AdditionalInfo...)
}
