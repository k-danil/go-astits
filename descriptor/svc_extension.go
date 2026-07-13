package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

// SVCExtension is the MPEG-2 systems SVC extension descriptor (ISO/IEC 13818-1).
type SVCExtension struct {
	Header              Header `json:"_header"`
	Width               uint16 `json:"width"`
	Height              uint16 `json:"height"`
	FrameRate           uint16 `json:"frame_rate"`      // frames per 256 seconds
	AverageBitrate      uint16 `json:"average_bitrate"` // kbit/s
	MaximumBitrate      uint16 `json:"maximum_bitrate"` // kbit/s
	DependencyID        uint8  `json:"dependency_id"`
	QualityIDStart      uint8  `json:"quality_id_start"`
	QualityIDEnd        uint8  `json:"quality_id_end"`
	TemporalIDStart     uint8  `json:"temporal_id_start"`
	TemporalIDEnd       uint8  `json:"temporal_id_end"`
	NoSEINALUnitPresent bool   `json:"no_sei_nal_unit_present"`
}

func newDescriptorSVCExtension(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(13); err != nil || len(bs) < 13 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &SVCExtension{
		Header:              h,
		Width:               binary.BigEndian.Uint16(bs[0:2]),
		Height:              binary.BigEndian.Uint16(bs[2:4]),
		FrameRate:           binary.BigEndian.Uint16(bs[4:6]),
		AverageBitrate:      binary.BigEndian.Uint16(bs[6:8]),
		MaximumBitrate:      binary.BigEndian.Uint16(bs[8:10]),
		DependencyID:        bs[10] >> 5,
		QualityIDStart:      bs[11] >> 4,
		QualityIDEnd:        bs[11] & 0x0f,
		TemporalIDStart:     bs[12] >> 5,
		TemporalIDEnd:       bs[12] >> 2 & 0x07,
		NoSEINALUnitPresent: bs[12]&0x02 > 0,
	}
	dd = d
	return
}

func (*SVCExtension) CalcLength() int {
	return 13
}

func (d *SVCExtension) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst,
		byte(d.Width>>8), byte(d.Width),
		byte(d.Height>>8), byte(d.Height),
		byte(d.FrameRate>>8), byte(d.FrameRate),
		byte(d.AverageBitrate>>8), byte(d.AverageBitrate),
		byte(d.MaximumBitrate>>8), byte(d.MaximumBitrate),
		d.DependencyID&0x07<<5|0x1f,
		d.QualityIDStart&0x0f<<4|d.QualityIDEnd&0x0f,
		d.TemporalIDStart&0x07<<5|d.TemporalIDEnd&0x07<<2|util.B2U(d.NoSEINALUnitPresent)<<1|0x01,
	)
	return dst
}
