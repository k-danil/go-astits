package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

// EnhancedAC3 represents an enhanced AC3 descriptor
// Chapter: Annex D | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type EnhancedAC3 struct {
	Header           Header
	AdditionalInfo   []byte
	ASVC             uint8
	BSID             uint8
	ComponentType    uint8
	HasASVC          bool
	HasBSID          bool
	HasComponentType bool
	HasMainID        bool
	HasSubStream1    bool
	HasSubStream2    bool
	HasSubStream3    bool
	MainID           uint8
	MixInfoExists    bool
	SubStream1       uint8
	SubStream2       uint8
	SubStream3       uint8
}

func newDescriptorEnhancedAC3(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	d := &EnhancedAC3{
		Header:           h,
		HasASVC:          uint8(b&0x10) > 0,
		HasBSID:          uint8(b&0x40) > 0,
		HasComponentType: uint8(b&0x80) > 0,
		HasMainID:        uint8(b&0x20) > 0,
		HasSubStream1:    uint8(b&0x4) > 0,
		HasSubStream2:    uint8(b&0x2) > 0,
		HasSubStream3:    uint8(b&0x1) > 0,
		MixInfoExists:    uint8(b&0x8) > 0,
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
	ret := 1 // flags
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
