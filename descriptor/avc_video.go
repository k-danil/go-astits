package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

// AVCVideo represents an AVC video descriptor
// No doc found unfortunately, basing the implementation on https://github.com/gfto/bitstream/blob/master/mpeg/psi/desc_28.h
type AVCVideo struct {
	Header               Header
	AVC24HourPictureFlag bool
	AVCStillPresent      bool
	CompatibleFlags      uint8
	ConstraintSet0Flag   bool
	ConstraintSet1Flag   bool
	ConstraintSet2Flag   bool
	LevelIDC             uint8
	ProfileIDC           uint8
}

func newDescriptorAVCVideo(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	// Init
	d := &AVCVideo{
		Header: h,
	}
	dd = d

	// Get next byte
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Profile idc
	d.ProfileIDC = b

	// Get next byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Flags
	d.ConstraintSet0Flag = b&0x80 > 0
	d.ConstraintSet1Flag = b&0x40 > 0
	d.ConstraintSet2Flag = b&0x20 > 0
	d.CompatibleFlags = b & 0x1f

	// Get next byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Level idc
	d.LevelIDC = b

	// Get next byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// AVC still present
	d.AVCStillPresent = b&0x80 > 0

	// AVC 24 hour picture flag
	d.AVC24HourPictureFlag = b&0x40 > 0
	return
}

func (*AVCVideo) CalcLength() int {
	return 4
}

func (d *AVCVideo) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, d.ProfileIDC)
	dst = append(dst, util.B2U(d.ConstraintSet0Flag)<<7|util.B2U(d.ConstraintSet1Flag)<<6|util.B2U(d.ConstraintSet2Flag)<<5|d.CompatibleFlags&0x1f)
	dst = append(dst, d.LevelIDC)
	return append(dst, util.B2U(d.AVCStillPresent)<<7|util.B2U(d.AVC24HourPictureFlag)<<6|0x3f)
}
