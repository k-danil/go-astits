package ext

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// range_type values (EN 300 468 Table 153)
const (
	VideoDepthRangeProductionDisparityHint = 0x00
	VideoDepthRangeMultiRegionSEI          = 0x01
)

// VideoDepthRange represents a video depth range extension descriptor:
// the intended depth range of plano-stereoscopic 3D video, so receivers can
// place graphics. For RangeType 0 the two disparity hints apply; for types >= 2
// the raw RangeSelector bytes apply.
// Chapter: 6.4.15 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type VideoDepthRange struct {
	Ranges []DepthRange
}

// DepthRange is one depth-range entry. VideoMaxDisparityHint /
// VideoMinDisparityHint (raw 12-bit signed values) apply for RangeType 0.
type DepthRange struct {
	RangeSelector         []byte
	VideoMaxDisparityHint uint16
	VideoMinDisparityHint uint16
	RangeType             uint8
}

func parseVideoDepthRange(i *bytesiter.Iterator, offsetEnd int) (d *VideoDepthRange, err error) {
	d = &VideoDepthRange{}

	for i.Offset() < offsetEnd {
		var rng DepthRange
		var bs []byte
		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		rng.RangeType = bs[0]
		rangeLength := int(bs[1])

		switch rng.RangeType {
		case VideoDepthRangeProductionDisparityHint:
			if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
				err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
				return
			}
			rng.VideoMaxDisparityHint = uint16(bs[0])<<4 | uint16(bs[1])>>4
			rng.VideoMinDisparityHint = uint16(bs[1]&0x0f)<<8 | uint16(bs[2])
		case VideoDepthRangeMultiRegionSEI:
		default:
			if rng.RangeSelector, err = i.NextBytes(rangeLength); err != nil {
				err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
				return
			}
		}
		d.Ranges = append(d.Ranges, rng)
	}
	return
}

func (rng *DepthRange) rangeLength() int {
	switch rng.RangeType {
	case VideoDepthRangeProductionDisparityHint:
		return 3
	case VideoDepthRangeMultiRegionSEI:
		return 0
	default:
		return len(rng.RangeSelector)
	}
}

func (d *VideoDepthRange) CalcLength() (n int) {
	for idx := range d.Ranges {
		n += 2 + d.Ranges[idx].rangeLength()
	}
	return
}

func (d *VideoDepthRange) Append(dst []byte) []byte {
	for idx := range d.Ranges {
		rng := &d.Ranges[idx]
		dst = append(dst, rng.RangeType, uint8(rng.rangeLength()))
		switch rng.RangeType {
		case VideoDepthRangeProductionDisparityHint:
			dst = append(dst,
				byte(rng.VideoMaxDisparityHint>>4),
				byte(rng.VideoMaxDisparityHint&0x0f)<<4|byte(rng.VideoMinDisparityHint>>8&0x0f),
				byte(rng.VideoMinDisparityHint))
		case VideoDepthRangeMultiRegionSEI:
		default:
			dst = append(dst, rng.RangeSelector...)
		}
	}
	return dst
}
