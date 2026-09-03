package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
	"github.com/k-danil/go-astits/v3/internal/util"
)

type EnhancedAC3 struct {
	Header           Header `json:"_header"`
	AdditionalInfo   []byte `json:"additional_info_byte"`
	ASVC             uint8  `json:"asvc"`
	BSID             uint8  `json:"bsid"`
	ComponentType    uint8  `json:"component_type"`
	HasASVC          bool   `json:"asvc_flag"`
	HasBSID          bool   `json:"bsid_flag"`
	HasComponentType bool   `json:"component_type_flag"`
	HasMainID        bool   `json:"mainid_flag"`
	HasSubStream1    bool   `json:"substream1_flag"`
	HasSubStream2    bool   `json:"substream2_flag"`
	HasSubStream3    bool   `json:"substream3_flag"`
	MainID           uint8  `json:"mainid"`
	MixInfoExists    bool   `json:"mixinfoexists"`
	SubStream1       uint8  `json:"substream1"`
	SubStream2       uint8  `json:"substream2"`
	SubStream3       uint8  `json:"substream3"`
}

func newDescriptorEnhancedAC3(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	d := &EnhancedAC3{
		Header:           h,
		HasASVC:          b&0x10 > 0,
		HasBSID:          b&0x40 > 0,
		HasComponentType: b&0x80 > 0,
		HasMainID:        b&0x20 > 0,
		HasSubStream1:    b&0x4 > 0,
		HasSubStream2:    b&0x2 > 0,
		HasSubStream3:    b&0x1 > 0,
		MixInfoExists:    b&0x8 > 0,
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

	if d.HasSubStream1 {
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.SubStream1 = b
	}

	if d.HasSubStream2 {
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.SubStream2 = b
	}

	if d.HasSubStream3 {
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.SubStream3 = b
	}

	if i.Offset() < offsetEnd {
		if d.AdditionalInfo, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *EnhancedAC3) CalcLength() int {
	ret := 1
	ret += int(util.B2U(d.HasComponentType))
	ret += int(util.B2U(d.HasBSID))
	ret += int(util.B2U(d.HasMainID))
	ret += int(util.B2U(d.HasASVC))
	ret += int(util.B2U(d.HasSubStream1))
	ret += int(util.B2U(d.HasSubStream2))
	ret += int(util.B2U(d.HasSubStream3))
	ret += len(d.AdditionalInfo)

	return ret
}

func (d *EnhancedAC3) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, util.B2U(d.HasComponentType)<<7|util.B2U(d.HasBSID)<<6|util.B2U(d.HasMainID)<<5|util.B2U(d.HasASVC)<<4|
		util.B2U(d.MixInfoExists)<<3|util.B2U(d.HasSubStream1)<<2|util.B2U(d.HasSubStream2)<<1|util.B2U(d.HasSubStream3))

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
	if d.HasSubStream1 {
		dst = append(dst, d.SubStream1)
	}
	if d.HasSubStream2 {
		dst = append(dst, d.SubStream2)
	}
	if d.HasSubStream3 {
		dst = append(dst, d.SubStream3)
	}

	return append(dst, d.AdditionalInfo...)
}
