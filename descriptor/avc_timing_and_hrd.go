package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

// AVCTimingAndHRD is the MPEG-2 systems AVC timing and HRD descriptor (ISO/IEC 13818-1).
type AVCTimingAndHRD struct {
	N                           uint32
	K                           uint32
	NumUnitsInTick              uint32
	Header                      Header
	HRDManagementValid          bool
	PictureAndTimingInfoPresent bool
	Is90kHz                     bool
	FixedFrameRate              bool
	TemporalPOC                 bool
	PictureToDisplayConversion  bool
}

func newDescriptorAVCTimingAndHRD(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	d := &AVCTimingAndHRD{
		Header:                      h,
		HRDManagementValid:          b&0x80 > 0,
		PictureAndTimingInfoPresent: b&0x01 > 0,
	}
	dd = d

	if d.PictureAndTimingInfoPresent {
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.Is90kHz = b&0x80 > 0

		if !d.Is90kHz {
			var bs []byte
			if bs, err = i.NextBytesNoCopy(8); err != nil || len(bs) < 8 {
				err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
				return
			}
			d.N = binary.BigEndian.Uint32(bs)
			d.K = binary.BigEndian.Uint32(bs[4:])
		}

		var bs []byte
		if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		d.NumUnitsInTick = binary.BigEndian.Uint32(bs)
	}

	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.FixedFrameRate = b&0x80 > 0
	d.TemporalPOC = b&0x40 > 0
	d.PictureToDisplayConversion = b&0x20 > 0

	return
}

func (d *AVCTimingAndHRD) CalcLength() int {
	ret := 2
	if d.PictureAndTimingInfoPresent {
		ret += 5
		if !d.Is90kHz {
			ret += 8
		}
	}
	return ret
}

func (d *AVCTimingAndHRD) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, util.B2U(d.HRDManagementValid)<<7|0x7e|util.B2U(d.PictureAndTimingInfoPresent))

	if d.PictureAndTimingInfoPresent {
		dst = append(dst, util.B2U(d.Is90kHz)<<7|0x7f)
		if !d.Is90kHz {
			dst = append(dst,
				byte(d.N>>24), byte(d.N>>16), byte(d.N>>8), byte(d.N),
				byte(d.K>>24), byte(d.K>>16), byte(d.K>>8), byte(d.K))
		}
		dst = append(dst, byte(d.NumUnitsInTick>>24), byte(d.NumUnitsInTick>>16), byte(d.NumUnitsInTick>>8), byte(d.NumUnitsInTick))
	}

	return append(dst, util.B2U(d.FixedFrameRate)<<7|util.B2U(d.TemporalPOC)<<6|util.B2U(d.PictureToDisplayConversion)<<5|0x1f)
}
