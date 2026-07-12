package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

// Hierarchy is the MPEG-2 systems hierarchy_descriptor (ISO/IEC 13818-1).
type Hierarchy struct {
	Header                      Header
	HierarchyType               uint8
	HierarchyLayerIndex         uint8
	HierarchyEmbeddedLayerIndex uint8
	HierarchyChannel            uint8
	NoViewScalabilityFlag       bool
	NoTemporalScalabilityFlag   bool
	NoSpatialScalabilityFlag    bool
	NoQualityScalabilityFlag    bool
	TREFPresentFlag             bool
}

func newDescriptorHierarchy(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &Hierarchy{
		Header:                      h,
		NoViewScalabilityFlag:       bs[0]&0x80 > 0,
		NoTemporalScalabilityFlag:   bs[0]&0x40 > 0,
		NoSpatialScalabilityFlag:    bs[0]&0x20 > 0,
		NoQualityScalabilityFlag:    bs[0]&0x10 > 0,
		HierarchyType:               bs[0] & 0x0f,
		HierarchyLayerIndex:         bs[1] & 0x3f,
		TREFPresentFlag:             bs[2]&0x80 > 0,
		HierarchyEmbeddedLayerIndex: bs[2] & 0x3f,
		HierarchyChannel:            bs[3] & 0x3f,
	}
	dd = d
	return
}

func (*Hierarchy) CalcLength() int {
	return 4
}

func (d *Hierarchy) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst,
		util.B2U(d.NoViewScalabilityFlag)<<7|util.B2U(d.NoTemporalScalabilityFlag)<<6|util.B2U(d.NoSpatialScalabilityFlag)<<5|util.B2U(d.NoQualityScalabilityFlag)<<4|d.HierarchyType&0x0f,
		0xc0|d.HierarchyLayerIndex&0x3f,
		util.B2U(d.TREFPresentFlag)<<7|0x40|d.HierarchyEmbeddedLayerIndex&0x3f,
		0xc0|d.HierarchyChannel&0x3f,
	)
}
