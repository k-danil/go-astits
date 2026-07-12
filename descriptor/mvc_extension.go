package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

// MVCExtension is the MPEG-2 systems MVC_extension_descriptor (ISO/IEC 13818-1).
type MVCExtension struct {
	AverageBitrate            uint16 // in kbit/s
	MaximumBitrate            uint16 // in kbit/s
	ViewOrderIndexMin         uint16
	ViewOrderIndexMax         uint16
	Header                    Header
	TemporalIDStart           uint8
	TemporalIDEnd             uint8
	ViewAssociationNotPresent bool
	BaseViewIsLeftEyeview     bool
	NoSEINALUnitPresent       bool
	NoPrefixNALUnitPresent    bool
}

func newDescriptorMVCExtension(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(8); err != nil || len(bs) < 8 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &MVCExtension{
		Header:                    h,
		AverageBitrate:            uint16(bs[0])<<8 | uint16(bs[1]),
		MaximumBitrate:            uint16(bs[2])<<8 | uint16(bs[3]),
		ViewAssociationNotPresent: bs[4]&0x80 > 0,
		BaseViewIsLeftEyeview:     bs[4]&0x40 > 0,
		ViewOrderIndexMin:         uint16(bs[4]&0x0f)<<6 | uint16(bs[5]>>2),
		ViewOrderIndexMax:         uint16(bs[5]&0x03)<<8 | uint16(bs[6]),
		TemporalIDStart:           bs[7] >> 5 & 0x7,
		TemporalIDEnd:             bs[7] >> 2 & 0x7,
		NoSEINALUnitPresent:       bs[7]&0x02 > 0,
		NoPrefixNALUnitPresent:    bs[7]&0x01 > 0,
	}
	dd = d
	return
}

func (*MVCExtension) CalcLength() int {
	return 8
}

func (d *MVCExtension) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, byte(d.AverageBitrate>>8), byte(d.AverageBitrate))
	dst = append(dst, byte(d.MaximumBitrate>>8), byte(d.MaximumBitrate))

	vMin := d.ViewOrderIndexMin & 0x3ff
	vMax := d.ViewOrderIndexMax & 0x3ff

	dst = append(dst, util.B2U(d.ViewAssociationNotPresent)<<7|util.B2U(d.BaseViewIsLeftEyeview)<<6|0x3<<4|byte(vMin>>6)&0x0f)
	dst = append(dst, byte(vMin&0x3f)<<2|byte(vMax>>8)&0x3)
	dst = append(dst, byte(vMax))
	dst = append(dst, d.TemporalIDStart&0x7<<5|d.TemporalIDEnd&0x7<<2|util.B2U(d.NoSEINALUnitPresent)<<1|util.B2U(d.NoPrefixNALUnitPresent))
	return dst
}
