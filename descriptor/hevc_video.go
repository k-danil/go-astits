package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

// HEVCVideo is the MPEG-2 systems HEVC_video_descriptor (ISO/IEC 13818-1).
type HEVCVideo struct {
	Copied44Bits                   uint64
	ProfileCompatibilityIndication uint32
	Header                         Header
	ProfileSpace                   uint8
	ProfileIDC                     uint8
	LevelIDC                       uint8
	HDRWCGIdc                      uint8
	TemporalIDMin                  uint8
	TemporalIDMax                  uint8
	TierFlag                       bool
	ProgressiveSourceFlag          bool
	InterlacedSourceFlag           bool
	NonPackedConstraintFlag        bool
	FrameOnlyConstraintFlag        bool
	TemporalLayerSubsetFlag        bool
	HEVCStillPresentFlag           bool
	HEVC24hrPicturePresentFlag     bool
	SubPicHRDParamsNotPresentFlag  bool
}

func newDescriptorHEVCVideo(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(13); err != nil || len(bs) < 13 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &HEVCVideo{
		Header:                         h,
		ProfileSpace:                   bs[0] >> 6 & 0x3,
		TierFlag:                       bs[0]&0x20 > 0,
		ProfileIDC:                     bs[0] & 0x1f,
		ProfileCompatibilityIndication: binary.BigEndian.Uint32(bs[1:5]),
		ProgressiveSourceFlag:          bs[5]&0x80 > 0,
		InterlacedSourceFlag:           bs[5]&0x40 > 0,
		NonPackedConstraintFlag:        bs[5]&0x20 > 0,
		FrameOnlyConstraintFlag:        bs[5]&0x10 > 0,
		Copied44Bits:                   uint64(bs[5]&0x0f)<<40 | uint64(bs[6])<<32 | uint64(bs[7])<<24 | uint64(bs[8])<<16 | uint64(bs[9])<<8 | uint64(bs[10]),
		LevelIDC:                       bs[11],
		TemporalLayerSubsetFlag:        bs[12]&0x80 > 0,
		HEVCStillPresentFlag:           bs[12]&0x40 > 0,
		HEVC24hrPicturePresentFlag:     bs[12]&0x20 > 0,
		SubPicHRDParamsNotPresentFlag:  bs[12]&0x10 > 0,
		HDRWCGIdc:                      bs[12] & 0x3,
	}
	dd = d

	if d.TemporalLayerSubsetFlag {
		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		d.TemporalIDMin = bs[0] >> 5 & 0x7
		d.TemporalIDMax = bs[1] >> 5 & 0x7
	}
	return
}

func (d *HEVCVideo) CalcLength() int {
	ret := 13
	if d.TemporalLayerSubsetFlag {
		ret += 2
	}
	return ret
}

func (d *HEVCVideo) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, d.ProfileSpace&0x3<<6|util.B2U(d.TierFlag)<<5|d.ProfileIDC&0x1f)
	dst = append(dst, byte(d.ProfileCompatibilityIndication>>24), byte(d.ProfileCompatibilityIndication>>16), byte(d.ProfileCompatibilityIndication>>8), byte(d.ProfileCompatibilityIndication))

	c := d.Copied44Bits & (1<<44 - 1)
	dst = append(dst,
		util.B2U(d.ProgressiveSourceFlag)<<7|util.B2U(d.InterlacedSourceFlag)<<6|util.B2U(d.NonPackedConstraintFlag)<<5|util.B2U(d.FrameOnlyConstraintFlag)<<4|byte(c>>40)&0x0f,
		byte(c>>32), byte(c>>24), byte(c>>16), byte(c>>8), byte(c))
	dst = append(dst, d.LevelIDC)
	dst = append(dst, util.B2U(d.TemporalLayerSubsetFlag)<<7|util.B2U(d.HEVCStillPresentFlag)<<6|util.B2U(d.HEVC24hrPicturePresentFlag)<<5|util.B2U(d.SubPicHRDParamsNotPresentFlag)<<4|0x3<<2|d.HDRWCGIdc&0x3)

	if d.TemporalLayerSubsetFlag {
		dst = append(dst, d.TemporalIDMin&0x7<<5|0x1f, d.TemporalIDMax&0x7<<5|0x1f)
	}
	return dst
}
