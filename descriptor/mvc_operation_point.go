package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

// MVCOperationPoint is the MPEG-2 systems MVC_operation_point_descriptor (ISO/IEC 13818-1).
type MVCOperationPoint struct {
	Levels             []MVCOperationPointLevel
	Header             Header
	ProfileIDC         uint8
	AVCCompatibleFlags uint8
	ConstraintSet0Flag bool
	ConstraintSet1Flag bool
	ConstraintSet2Flag bool
	ConstraintSet3Flag bool
	ConstraintSet4Flag bool
	ConstraintSet5Flag bool
}

type MVCOperationPointLevel struct {
	OperationPoints []MVCOperationPointEntry
	LevelIDC        uint8
}

type MVCOperationPointEntry struct {
	ESReferences         []uint8
	NumTargetOutputViews uint8
	ApplicableTemporalID uint8
}

func newDescriptorMVCOperationPoint(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &MVCOperationPoint{
		Header:             h,
		ProfileIDC:         bs[0],
		ConstraintSet0Flag: bs[1]&0x80 > 0,
		ConstraintSet1Flag: bs[1]&0x40 > 0,
		ConstraintSet2Flag: bs[1]&0x20 > 0,
		ConstraintSet3Flag: bs[1]&0x10 > 0,
		ConstraintSet4Flag: bs[1]&0x08 > 0,
		ConstraintSet5Flag: bs[1]&0x04 > 0,
		AVCCompatibleFlags: bs[1] & 0x03,
	}
	dd = d

	d.Levels = make([]MVCOperationPointLevel, bs[2])
	for li := range d.Levels {
		level := &d.Levels[li]
		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		level.LevelIDC = bs[0]
		level.OperationPoints = make([]MVCOperationPointEntry, bs[1])
		for oi := range level.OperationPoints {
			op := &level.OperationPoints[oi]
			if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
				err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
				return
			}
			op.ApplicableTemporalID = bs[0] & 0x07
			op.NumTargetOutputViews = bs[1]
			op.ESReferences = make([]uint8, bs[2])
			if bs, err = i.NextBytesNoCopy(len(op.ESReferences)); err != nil || len(bs) < len(op.ESReferences) {
				err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
				return
			}
			for ei := range op.ESReferences {
				op.ESReferences[ei] = bs[ei] & 0x3f
			}
		}
	}
	return
}

func (d *MVCOperationPoint) CalcLength() int {
	ret := 3
	for li := range d.Levels {
		ret += 2
		for oi := range d.Levels[li].OperationPoints {
			ret += 3 + len(d.Levels[li].OperationPoints[oi].ESReferences)
		}
	}
	return ret
}

func (d *MVCOperationPoint) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, d.ProfileIDC)
	dst = append(dst, util.B2U(d.ConstraintSet0Flag)<<7|util.B2U(d.ConstraintSet1Flag)<<6|
		util.B2U(d.ConstraintSet2Flag)<<5|util.B2U(d.ConstraintSet3Flag)<<4|
		util.B2U(d.ConstraintSet4Flag)<<3|util.B2U(d.ConstraintSet5Flag)<<2|d.AVCCompatibleFlags&0x03)
	dst = append(dst, uint8(len(d.Levels)))

	for li := range d.Levels {
		level := &d.Levels[li]
		dst = append(dst, level.LevelIDC, uint8(len(level.OperationPoints)))
		for oi := range level.OperationPoints {
			op := &level.OperationPoints[oi]
			dst = append(dst, 0xf8|op.ApplicableTemporalID&0x07, op.NumTargetOutputViews, uint8(len(op.ESReferences)))
			for _, es := range op.ESReferences {
				dst = append(dst, 0xc0|es&0x3f)
			}
		}
	}
	return dst
}
