package descriptor

import (
	"encoding/json"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

type ServiceType uint8

// Service types
// Chapter: 6.2.33, Table 87 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
const (
	ServiceTypeDigitalTelevisionService                    ServiceType = 0x01
	ServiceTypeDigitalRadioSoundService                    ServiceType = 0x02
	ServiceTypeTeletextService                             ServiceType = 0x03
	ServiceTypeNVODReferenceService                        ServiceType = 0x04
	ServiceTypeNVODTimeShiftedService                      ServiceType = 0x05
	ServiceTypeMosaicService                               ServiceType = 0x06
	ServiceTypeFMRadioService                              ServiceType = 0x07
	ServiceTypeDVBSRMService                               ServiceType = 0x08
	ServiceTypeAdvancedCodecDigitalRadioSoundService       ServiceType = 0x0a
	ServiceTypeH264AVCMosaicService                        ServiceType = 0x0b
	ServiceTypeDataBroadcastService                        ServiceType = 0x0c
	ServiceTypeCommonInterfaceUsage                        ServiceType = 0x0d
	ServiceTypeRCSMap                                      ServiceType = 0x0e
	ServiceTypeRCSFLS                                      ServiceType = 0x0f
	ServiceTypeDVBMHPService                               ServiceType = 0x10
	ServiceTypeMPEG2HDDigitalTelevisionService             ServiceType = 0x11
	ServiceTypeH264AVCSDDigitalTelevisionService           ServiceType = 0x16
	ServiceTypeH264AVCSDNVODTimeShiftedService             ServiceType = 0x17
	ServiceTypeH264AVCSDNVODReferenceService               ServiceType = 0x18
	ServiceTypeH264AVCHDDigitalTelevisionService           ServiceType = 0x19
	ServiceTypeH264AVCHDNVODTimeShiftedService             ServiceType = 0x1a
	ServiceTypeH264AVCHDNVODReferenceService               ServiceType = 0x1b
	ServiceTypeH264AVCStereoscopicHDDigitalTelevision      ServiceType = 0x1c
	ServiceTypeH264AVCStereoscopicHDNVODTimeShiftedService ServiceType = 0x1d
	ServiceTypeH264AVCStereoscopicHDNVODReferenceService   ServiceType = 0x1e
	ServiceTypeHEVCDigitalTelevisionService                ServiceType = 0x1f
)

var serviceTypeNames = map[ServiceType]string{
	ServiceTypeDigitalTelevisionService:                    "digital_television_service",
	ServiceTypeDigitalRadioSoundService:                    "digital_radio_sound_service",
	ServiceTypeTeletextService:                             "Teletext_service",
	ServiceTypeNVODReferenceService:                        "NVOD_reference_service",
	ServiceTypeNVODTimeShiftedService:                      "NVOD_time_shifted_service",
	ServiceTypeMosaicService:                               "mosaic_service",
	ServiceTypeFMRadioService:                              "FM_radio_service",
	ServiceTypeDVBSRMService:                               "DVB_SRM_service",
	ServiceTypeAdvancedCodecDigitalRadioSoundService:       "advanced_codec_digital_radio_sound_service",
	ServiceTypeH264AVCMosaicService:                        "H264_AVC_mosaic_service",
	ServiceTypeDataBroadcastService:                        "data_broadcast_service",
	ServiceTypeCommonInterfaceUsage:                        "reserved_for_common_interface_usage",
	ServiceTypeRCSMap:                                      "RCS_Map",
	ServiceTypeRCSFLS:                                      "RCS_FLS",
	ServiceTypeDVBMHPService:                               "DVB_MHP_service",
	ServiceTypeMPEG2HDDigitalTelevisionService:             "MPEG2_HD_digital_television_service",
	ServiceTypeH264AVCSDDigitalTelevisionService:           "H264_AVC_SD_digital_television_service",
	ServiceTypeH264AVCSDNVODTimeShiftedService:             "H264_AVC_SD_NVOD_time_shifted_service",
	ServiceTypeH264AVCSDNVODReferenceService:               "H264_AVC_SD_NVOD_reference_service",
	ServiceTypeH264AVCHDDigitalTelevisionService:           "H264_AVC_HD_digital_television_service",
	ServiceTypeH264AVCHDNVODTimeShiftedService:             "H264_AVC_HD_NVOD_time_shifted_service",
	ServiceTypeH264AVCHDNVODReferenceService:               "H264_AVC_HD_NVOD_reference_service",
	ServiceTypeH264AVCStereoscopicHDDigitalTelevision:      "H264_AVC_frame_compatible_plano_stereoscopic_HD_digital_television_service",
	ServiceTypeH264AVCStereoscopicHDNVODTimeShiftedService: "H264_AVC_frame_compatible_plano_stereoscopic_HD_NVOD_time_shifted_service",
	ServiceTypeH264AVCStereoscopicHDNVODReferenceService:   "H264_AVC_frame_compatible_plano_stereoscopic_HD_NVOD_reference_service",
	ServiceTypeHEVCDigitalTelevisionService:                "HEVC_digital_television_service",
}

func (t ServiceType) String() (s string) {
	var ok bool
	if s, ok = serviceTypeNames[t]; !ok {
		s = fmt.Sprintf("0x%02x", uint8(t))
	}
	return
}

func (t ServiceType) MarshalJSON() (b []byte, err error) {
	return json.Marshal(t.String())
}

func (t *ServiceType) UnmarshalJSON(b []byte) (err error) {
	*t, err = util.UnmarshalEnum(b, serviceTypeNames)
	return
}

// Service represents a service descriptor
// Chapter: 6.2.33 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type Service struct {
	Name     []byte      `json:"service_name"`
	Provider []byte      `json:"service_provider_name"`
	Header   Header      `json:"_header"`
	Type     ServiceType `json:"service_type"`
}

func newDescriptorService(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	d := &Service{
		Header: h,
		Type:   ServiceType(b),
	}
	dd = d

	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	providerLength := int(b)

	if d.Provider, err = i.NextBytes(providerLength); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	nameLength := int(b)

	if d.Name, err = i.NextBytes(nameLength); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *Service) CalcLength() int {
	return 3 + len(d.Name) + len(d.Provider) // type and lengths
}

func (d *Service) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, uint8(d.Type), uint8(len(d.Provider)))
	dst = append(dst, d.Provider...)
	dst = append(dst, uint8(len(d.Name)))
	return append(dst, d.Name...)
}
