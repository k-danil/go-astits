package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

// AVCVideo represents an AVC video descriptor
// Chapter: 2.6.64 | Link: doc/h222.0-201703-iso13818-1.pdf (ITU-T H.222.0 = ISO/IEC 13818-1)
type AVCVideo struct {
	Header                        Header `json:"_header"`
	AVC24HourPictureFlag          bool   `json:"AVC_24_hour_picture_flag"`
	AVCStillPresent               bool   `json:"AVC_still_present"`
	FramePackingSEINotPresentFlag bool   `json:"Frame_Packing_SEI_not_present_flag"`
	ConstraintSet0Flag            bool   `json:"constraint_set0_flag"`
	ConstraintSet1Flag            bool   `json:"constraint_set1_flag"`
	ConstraintSet2Flag            bool   `json:"constraint_set2_flag"`
	ConstraintSet3Flag            bool   `json:"constraint_set3_flag"`
	ConstraintSet4Flag            bool   `json:"constraint_set4_flag"`
	ConstraintSet5Flag            bool   `json:"constraint_set5_flag"`
	CompatibleFlags               uint8  `json:"AVC_compatible_flags"`
	LevelIDC                      uint8  `json:"level_idc"`
	ProfileIDC                    uint8  `json:"profile_idc"`
}

func newDescriptorAVCVideo(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	d := &AVCVideo{
		Header: h,
	}
	dd = d

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.ProfileIDC = b

	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.ConstraintSet0Flag = b&0x80 > 0
	d.ConstraintSet1Flag = b&0x40 > 0
	d.ConstraintSet2Flag = b&0x20 > 0
	d.ConstraintSet3Flag = b&0x10 > 0
	d.ConstraintSet4Flag = b&0x08 > 0
	d.ConstraintSet5Flag = b&0x04 > 0
	d.CompatibleFlags = b & 0x03

	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.LevelIDC = b

	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.AVCStillPresent = b&0x80 > 0
	d.AVC24HourPictureFlag = b&0x40 > 0
	d.FramePackingSEINotPresentFlag = b&0x20 > 0
	return
}

func (*AVCVideo) CalcLength() int {
	return 4
}

func (d *AVCVideo) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, d.ProfileIDC)
	dst = append(dst, util.B2U(d.ConstraintSet0Flag)<<7|util.B2U(d.ConstraintSet1Flag)<<6|
		util.B2U(d.ConstraintSet2Flag)<<5|util.B2U(d.ConstraintSet3Flag)<<4|
		util.B2U(d.ConstraintSet4Flag)<<3|util.B2U(d.ConstraintSet5Flag)<<2|d.CompatibleFlags&0x03)
	dst = append(dst, d.LevelIDC)
	return append(dst, util.B2U(d.AVCStillPresent)<<7|util.B2U(d.AVC24HourPictureFlag)<<6|
		util.B2U(d.FramePackingSEINotPresentFlag)<<5|0x1f)
}
