package descriptor

import (
	"encoding/json"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

type HierarchyType uint8

// hierarchy_type values (ISO/IEC 13818-1 Table 2-50)
const (
	HierarchyTypeSpatialScalability    HierarchyType = 1
	HierarchyTypeSNRScalability        HierarchyType = 2
	HierarchyTypeTemporalScalability   HierarchyType = 3
	HierarchyTypeDataPartitioning      HierarchyType = 4
	HierarchyTypeExtensionBitstream    HierarchyType = 5
	HierarchyTypePrivateStream         HierarchyType = 6
	HierarchyTypeMultiViewProfile      HierarchyType = 7
	HierarchyTypeCombinedScalability   HierarchyType = 8
	HierarchyTypeMVCVideoSubBitstream  HierarchyType = 9
	HierarchyTypeAuxiliaryPictureLayer HierarchyType = 10
	HierarchyTypeBaseLayer             HierarchyType = 15
)

var hierarchyTypeNames = map[HierarchyType]string{
	HierarchyTypeSpatialScalability:    "spatial_scalability",
	HierarchyTypeSNRScalability:        "SNR_scalability",
	HierarchyTypeTemporalScalability:   "temporal_scalability",
	HierarchyTypeDataPartitioning:      "data_partitioning",
	HierarchyTypeExtensionBitstream:    "extension_bitstream",
	HierarchyTypePrivateStream:         "private_stream",
	HierarchyTypeMultiViewProfile:      "multi_view_profile",
	HierarchyTypeCombinedScalability:   "combined_scalability_or_MV_HEVC_sub_partition",
	HierarchyTypeMVCVideoSubBitstream:  "MVC_or_MVCD_video_sub_bitstream",
	HierarchyTypeAuxiliaryPictureLayer: "auxiliary_picture_layer",
	HierarchyTypeBaseLayer:             "base_layer",
}

func (t HierarchyType) String() (s string) {
	var ok bool
	if s, ok = hierarchyTypeNames[t]; !ok {
		s = fmt.Sprintf("0x%02x", uint8(t))
	}
	return
}

func (t HierarchyType) MarshalJSON() (b []byte, err error) {
	return json.Marshal(t.String())
}

func (t *HierarchyType) UnmarshalJSON(b []byte) (err error) {
	*t, err = util.UnmarshalEnum(b, hierarchyTypeNames)
	return
}

// Hierarchy is the MPEG-2 systems hierarchy_descriptor (ISO/IEC 13818-1).
type Hierarchy struct {
	Header                      Header        `json:"_header"`
	HierarchyType               HierarchyType `json:"hierarchy_type"`
	HierarchyLayerIndex         uint8         `json:"hierarchy_layer_index"`
	HierarchyEmbeddedLayerIndex uint8         `json:"hierarchy_embedded_layer_index"`
	HierarchyChannel            uint8         `json:"hierarchy_channel"`
	NoViewScalabilityFlag       bool          `json:"no_view_scalability_flag"`
	NoTemporalScalabilityFlag   bool          `json:"no_temporal_scalability_flag"`
	NoSpatialScalabilityFlag    bool          `json:"no_spatial_scalability_flag"`
	NoQualityScalabilityFlag    bool          `json:"no_quality_scalability_flag"`
	TREFPresentFlag             bool          `json:"tref_present_flag"`
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
		HierarchyType:               HierarchyType(bs[0] & 0x0f),
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
		util.B2U(d.NoViewScalabilityFlag)<<7|util.B2U(d.NoTemporalScalabilityFlag)<<6|util.B2U(d.NoSpatialScalabilityFlag)<<5|util.B2U(d.NoQualityScalabilityFlag)<<4|uint8(d.HierarchyType)&0x0f,
		0xc0|d.HierarchyLayerIndex&0x3f,
		util.B2U(d.TREFPresentFlag)<<7|0x40|d.HierarchyEmbeddedLayerIndex&0x3f,
		0xc0|d.HierarchyChannel&0x3f,
	)
}
