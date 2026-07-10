package descriptor

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func randBytes(r *rand.Rand, n int) []byte {
	bs := make([]byte, n)
	for i := range bs {
		bs[i] = uint8(r.UintN(256))
	}
	return bs
}

func randLang(r *rand.Rand) [3]byte {
	return [3]byte{uint8('a' + r.UintN(26)), uint8('a' + r.UintN(26)), uint8('a' + r.UintN(26))}
}

func randDVBTime(r *rand.Rand) time.Time {
	return time.Date(2000+int(r.UintN(40)), time.Month(1+r.UintN(12)), 1+int(r.UintN(28)),
		int(r.UintN(24)), int(r.UintN(60)), int(r.UintN(60)), 0, time.UTC)
}

func randDVBMinutes(r *rand.Rand) time.Duration {
	return time.Duration(r.UintN(24))*time.Hour + time.Duration(r.UintN(60))*time.Minute
}

var roundtripGenerators = map[string]func(r *rand.Rand) Descriptor{
	"AC3": func(r *rand.Rand) Descriptor {
		d := &AC3{Header: Header{Tag: TagAC3},
			HasComponentType: r.UintN(2) == 1, HasBSID: r.UintN(2) == 1,
			HasMainID: r.UintN(2) == 1, HasASVC: r.UintN(2) == 1,
			AdditionalInfo: randBytes(r, int(r.UintN(10)))}
		if d.HasComponentType {
			d.ComponentType = uint8(r.UintN(256))
		}
		if d.HasBSID {
			d.BSID = uint8(r.UintN(256))
		}
		if d.HasMainID {
			d.MainID = uint8(r.UintN(256))
		}
		if d.HasASVC {
			d.ASVC = uint8(r.UintN(256))
		}
		return d
	},
	"EnhancedAC3": func(r *rand.Rand) Descriptor {
		d := &EnhancedAC3{Header: Header{Tag: TagEnhancedAC3},
			HasComponentType: r.UintN(2) == 1, HasBSID: r.UintN(2) == 1,
			HasMainID: r.UintN(2) == 1, HasASVC: r.UintN(2) == 1,
			MixInfoExists: r.UintN(2) == 1,
			HasSubStream1: r.UintN(2) == 1, HasSubStream2: r.UintN(2) == 1, HasSubStream3: r.UintN(2) == 1,
			AdditionalInfo: randBytes(r, int(r.UintN(10)))}
		if d.HasComponentType {
			d.ComponentType = uint8(r.UintN(256))
		}
		if d.HasBSID {
			d.BSID = uint8(r.UintN(256))
		}
		if d.HasMainID {
			d.MainID = uint8(r.UintN(256))
		}
		if d.HasASVC {
			d.ASVC = uint8(r.UintN(256))
		}
		if d.HasSubStream1 {
			d.SubStream1 = uint8(r.UintN(256))
		}
		if d.HasSubStream2 {
			d.SubStream2 = uint8(r.UintN(256))
		}
		if d.HasSubStream3 {
			d.SubStream3 = uint8(r.UintN(256))
		}
		return d
	},
	"AVCVideo": func(r *rand.Rand) Descriptor {
		return &AVCVideo{Header: Header{Tag: TagAVCVideo},
			ProfileIDC: uint8(r.UintN(256)), LevelIDC: uint8(r.UintN(256)),
			ConstraintSet0Flag: r.UintN(2) == 1, ConstraintSet1Flag: r.UintN(2) == 1,
			ConstraintSet2Flag: r.UintN(2) == 1, CompatibleFlags: uint8(r.UintN(32)),
			AVCStillPresent: r.UintN(2) == 1, AVC24HourPictureFlag: r.UintN(2) == 1}
	},
	"Component": func(r *rand.Rand) Descriptor {
		return &Component{Header: Header{Tag: TagComponent},
			StreamContentExt: uint8(r.UintN(16)), StreamContent: uint8(r.UintN(16)),
			ComponentType: uint8(r.UintN(256)), ComponentTag: uint8(r.UintN(256)),
			ISO639LanguageCode: randLang(r), Text: randBytes(r, int(r.UintN(16)))}
	},
	"Content": func(r *rand.Rand) Descriptor {
		d := &Content{Header: Header{Tag: TagContent}}
		for i := uint(0); i < 1+r.UintN(5); i++ {
			d.Items = append(d.Items, ContentItem{
				ContentNibbleLevel1: uint8(r.UintN(16)),
				ContentNibbleLevel2: uint8(r.UintN(16)),
				UserByte:            uint8(r.UintN(256))})
		}
		return d
	},
	"DataStreamAlignment": func(r *rand.Rand) Descriptor {
		return &DataStreamAlignment{Header: Header{Tag: TagDataStreamAlignment}, Type: uint8(r.UintN(256))}
	},
	"ExtendedEvent": func(r *rand.Rand) Descriptor {
		d := &ExtendedEvent{Header: Header{Tag: TagExtendedEvent},
			Number: uint8(r.UintN(16)), LastDescriptorNumber: uint8(r.UintN(16)),
			ISO639LanguageCode: randLang(r), Text: randBytes(r, int(r.UintN(16)))}
		for i := uint(0); i < r.UintN(3); i++ {
			d.Items = append(d.Items, ExtendedEventItem{
				Description: randBytes(r, int(r.UintN(10))),
				Content:     randBytes(r, int(r.UintN(10)))})
		}
		return d
	},
	"ExtensionSupplementaryAudio": func(r *rand.Rand) Descriptor {
		d := &Extension{Header: Header{Tag: TagExtension}, Tag: TagExtensionSupplementaryAudio,
			SupplementaryAudio: &ExtensionSupplementaryAudio{
				MixType:                 r.UintN(2) == 1,
				EditorialClassification: uint8(r.UintN(32)),
				HasLanguageCode:         r.UintN(2) == 1,
				PrivateData:             randBytes(r, int(r.UintN(8)))}}
		if d.SupplementaryAudio.HasLanguageCode {
			d.SupplementaryAudio.LanguageCode = randLang(r)
		}
		return d
	},
	"ISO639": func(r *rand.Rand) Descriptor {
		d := &ISO639LanguageAndAudioType{Header: Header{Tag: TagISO639LanguageAndAudioType}}
		for i := uint(0); i < 1+r.UintN(4); i++ {
			d.Items = append(d.Items, ISO639Item{Language: randLang(r), Type: uint8(r.UintN(256))})
		}
		return d
	},
	"LocalTimeOffset": func(r *rand.Rand) Descriptor {
		d := &LocalTimeOffset{Header: Header{Tag: TagLocalTimeOffset}}
		for i := uint(0); i < 1+r.UintN(3); i++ {
			d.Items = append(d.Items, LocalTimeOffsetItem{
				CountryCode:             randLang(r),
				CountryRegionID:         uint8(r.UintN(64)),
				LocalTimeOffsetPolarity: r.UintN(2) == 1,
				LocalTimeOffset:         randDVBMinutes(r),
				TimeOfChange:            randDVBTime(r),
				NextTimeOffset:          randDVBMinutes(r)})
		}
		return d
	},
	"MaximumBitrate": func(r *rand.Rand) Descriptor {
		return &MaximumBitrate{Header: Header{Tag: TagMaximumBitrate}, Bitrate: uint32(r.UintN(1<<22)) * 50}
	},
	"NetworkName": func(r *rand.Rand) Descriptor {
		return &NetworkName{Header: Header{Tag: TagNetworkName}, Name: randBytes(r, int(r.UintN(20)))}
	},
	"ParentalRating": func(r *rand.Rand) Descriptor {
		d := &ParentalRating{Header: Header{Tag: TagParentalRating}}
		for i := uint(0); i < 1+r.UintN(4); i++ {
			d.Items = append(d.Items, ParentalRatingItem{CountryCode: randLang(r), Rating: uint8(r.UintN(256))})
		}
		return d
	},
	"PrivateDataIndicator": func(r *rand.Rand) Descriptor {
		return &PrivateDataIndicator{Header: Header{Tag: TagPrivateDataIndicator}, Indicator: r.Uint32()}
	},
	"PrivateDataSpecifier": func(r *rand.Rand) Descriptor {
		return &PrivateDataSpecifier{Header: Header{Tag: TagPrivateDataSpecifier}, Specifier: r.Uint32()}
	},
	"Registration": func(r *rand.Rand) Descriptor {
		return &Registration{Header: Header{Tag: TagRegistration},
			FormatIdentifier: r.Uint32(), AdditionalIdentificationInfo: randBytes(r, int(r.UintN(10)))}
	},
	"Service": func(r *rand.Rand) Descriptor {
		return &Service{Header: Header{Tag: TagService}, Type: uint8(r.UintN(256)),
			Provider: randBytes(r, int(r.UintN(16))), Name: randBytes(r, int(r.UintN(16)))}
	},
	"ShortEvent": func(r *rand.Rand) Descriptor {
		return &ShortEvent{Header: Header{Tag: TagShortEvent}, Language: randLang(r),
			EventName: randBytes(r, int(r.UintN(16))), Text: randBytes(r, int(r.UintN(16)))}
	},
	"StreamIdentifier": func(r *rand.Rand) Descriptor {
		return &StreamIdentifier{Header: Header{Tag: TagStreamIdentifier}, ComponentTag: uint8(r.UintN(256))}
	},
	"Subtitling": func(r *rand.Rand) Descriptor {
		d := &Subtitling{Header: Header{Tag: TagSubtitling}}
		for i := uint(0); i < 1+r.UintN(4); i++ {
			d.Items = append(d.Items, SubtitlingItem{Language: randLang(r), Type: uint8(r.UintN(256)),
				CompositionPageID: uint16(r.UintN(1 << 16)), AncillaryPageID: uint16(r.UintN(1 << 16))})
		}
		return d
	},
	"Teletext": func(r *rand.Rand) Descriptor {
		d := &Teletext{Header: Header{Tag: TagTeletext}}
		for i := uint(0); i < 1+r.UintN(4); i++ {
			d.Items = append(d.Items, TeletextItem{Language: randLang(r), Type: uint8(r.UintN(32)),
				Magazine: uint8(r.UintN(8)), Page: uint8(r.UintN(100))})
		}
		return d
	},
	"UserDefined": func(r *rand.Rand) Descriptor {
		return &UserDefined{Header: Header{Tag: Tag(0x80 + r.UintN(0x7f))}, Data: randBytes(r, int(r.UintN(20)))}
	},
	"Unknown": func(r *rand.Rand) Descriptor {
		return &Unknown{Header: Header{Tag: Tag(0x30)}, Content: randBytes(r, int(r.UintN(20)))}
	},
	"VBIData": func(r *rand.Rand) Descriptor {
		d := &VBIData{Header: Header{Tag: TagVBIData}}
		for i := uint(0); i < 1+r.UintN(3); i++ {
			svc := VBIDataService{DataServiceID: VBIDataServiceIDEBUTeletext}
			for j := uint(0); j < 1+r.UintN(3); j++ {
				svc.Descriptors = append(svc.Descriptors, VBIDataDescriptor{
					FieldParity: r.UintN(2) == 1, LineOffset: uint8(r.UintN(32))})
			}
			d.Services = append(d.Services, svc)
		}
		return d
	},
	"BouquetName": func(r *rand.Rand) Descriptor {
		return &BouquetName{Header: Header{Tag: TagBouquetName}, Name: randBytes(r, int(r.UintN(20)))}
	},
	"CAIdentifier": func(r *rand.Rand) Descriptor {
		d := &CAIdentifier{Header: Header{Tag: TagCAIdentifier}}
		for i := uint(0); i < 1+r.UintN(4); i++ {
			d.SystemIDs = append(d.SystemIDs, uint16(r.UintN(1<<16)))
		}
		return d
	},
	"CountryAvailability": func(r *rand.Rand) Descriptor {
		d := &CountryAvailability{Header: Header{Tag: TagCountryAvailability}, AvailabilityFlag: r.UintN(2) == 1}
		for i := uint(0); i < 1+r.UintN(4); i++ {
			d.Countries = append(d.Countries, randLang(r))
		}
		return d
	},
	"ServiceList": func(r *rand.Rand) Descriptor {
		d := &ServiceList{Header: Header{Tag: TagServiceList}}
		for i := uint(0); i < 1+r.UintN(5); i++ {
			d.Items = append(d.Items, ServiceListItem{
				ServiceID: uint16(r.UintN(1 << 16)), ServiceType: uint8(r.UintN(256))})
		}
		return d
	},
	"ServiceMove": func(r *rand.Rand) Descriptor {
		return &ServiceMove{Header: Header{Tag: TagServiceMove},
			NewOriginalNetworkID: uint16(r.UintN(1 << 16)),
			NewTransportStreamID: uint16(r.UintN(1 << 16)),
			NewServiceID:         uint16(r.UintN(1 << 16))}
	},
	"ServiceAvailability": func(r *rand.Rand) Descriptor {
		d := &ServiceAvailability{Header: Header{Tag: TagServiceAvailability}, AvailabilityFlag: r.UintN(2) == 1}
		for i := uint(0); i < 1+r.UintN(4); i++ {
			d.CellIDs = append(d.CellIDs, uint16(r.UintN(1<<16)))
		}
		return d
	},
	"Stuffing": func(r *rand.Rand) Descriptor {
		return &Stuffing{Header: Header{Tag: TagStuffing}, Data: randBytes(r, 1+int(r.UintN(20)))}
	},
	"CableDeliverySystem": func(r *rand.Rand) Descriptor {
		return &CableDeliverySystem{Header: Header{Tag: TagCableDeliverySystem},
			Frequency: r.Uint32(), SymbolRate: uint32(r.UintN(1 << 28)),
			FECOuter: uint8(r.UintN(16)), Modulation: uint8(r.UintN(256)), FECInner: uint8(r.UintN(16))}
	},
	"SatelliteDeliverySystem": func(r *rand.Rand) Descriptor {
		return &SatelliteDeliverySystem{Header: Header{Tag: TagSatelliteDeliverySystem},
			Frequency: r.Uint32(), OrbitalPosition: uint16(r.UintN(1 << 16)),
			WestEastFlag: r.UintN(2) == 1, Polarization: uint8(r.UintN(4)), RollOff: uint8(r.UintN(4)),
			ModulationSystem: r.UintN(2) == 1, ModulationType: uint8(r.UintN(4)),
			SymbolRate: uint32(r.UintN(1 << 28)), FECInner: uint8(r.UintN(16))}
	},
	"S2SatelliteDeliverySystem": func(r *rand.Rand) Descriptor {
		return &S2SatelliteDeliverySystem{Header: Header{Tag: TagS2SatelliteDeliverySystem},
			ScramblingSequenceSelector: r.UintN(2) == 1, MultipleInputStreamFlag: r.UintN(2) == 1,
			BackwardsCompatibilityIndicator: r.UintN(2) == 1,
			ScramblingSequenceIndex:         uint32(r.UintN(1 << 18)), InputStreamIdentifier: uint8(r.UintN(256))}
	},
	"TerrestrialDeliverySystem": func(r *rand.Rand) Descriptor {
		return &TerrestrialDeliverySystem{Header: Header{Tag: TagTerrestrialDeliverySystem},
			CentreFrequency: r.Uint32(), Bandwidth: uint8(r.UintN(8)),
			Priority: r.UintN(2) == 1, TimeSlicingIndicator: r.UintN(2) == 1, MPEFECIndicator: r.UintN(2) == 1,
			Constellation: uint8(r.UintN(4)), HierarchyInformation: uint8(r.UintN(8)),
			CodeRateHPStream: uint8(r.UintN(8)), CodeRateLPStream: uint8(r.UintN(8)),
			GuardInterval: uint8(r.UintN(4)), TransmissionMode: uint8(r.UintN(4)),
			OtherFrequencyFlag: r.UintN(2) == 1}
	},
	"NVODReference": func(r *rand.Rand) Descriptor {
		d := &NVODReference{Header: Header{Tag: TagNVODReference}}
		for i := uint(0); i < 1+r.UintN(4); i++ {
			d.Items = append(d.Items, NVODReferenceItem{
				TransportStreamID: uint16(r.UintN(1 << 16)),
				OriginalNetworkID: uint16(r.UintN(1 << 16)),
				ServiceID:         uint16(r.UintN(1 << 16))})
		}
		return d
	},
	"PDC": func(r *rand.Rand) Descriptor {
		return &PDC{Header: Header{Tag: TagPDC}, ProgrammeIdentificationLabel: uint32(r.UintN(1 << 20))}
	},
	"Scrambling": func(r *rand.Rand) Descriptor {
		return &Scrambling{Header: Header{Tag: TagScrambling}, Mode: uint8(r.UintN(256))}
	},
	"TimeShiftedEvent": func(r *rand.Rand) Descriptor {
		return &TimeShiftedEvent{Header: Header{Tag: TagTimeShiftedEvent},
			ReferenceServiceID: uint16(r.UintN(1 << 16)), ReferenceEventID: uint16(r.UintN(1 << 16))}
	},
	"TimeShiftedService": func(r *rand.Rand) Descriptor {
		return &TimeShiftedService{Header: Header{Tag: TagTimeShiftedService},
			ReferenceServiceID: uint16(r.UintN(1 << 16))}
	},
	"TransportStream": func(r *rand.Rand) Descriptor {
		return &TransportStream{Header: Header{Tag: TagTransportStream}, Data: randBytes(r, 1+int(r.UintN(8)))}
	},
	"DSNG": func(r *rand.Rand) Descriptor {
		return &DSNG{Header: Header{Tag: TagDSNG}, Data: randBytes(r, 1+int(r.UintN(16)))}
	},
	"Linkage": func(r *rand.Rand) Descriptor {
		return &Linkage{Header: Header{Tag: TagLinkage},
			TransportStreamID: uint16(r.UintN(1 << 16)), OriginalNetworkID: uint16(r.UintN(1 << 16)),
			ServiceID: uint16(r.UintN(1 << 16)), LinkageType: uint8(r.UintN(256)),
			Data: randBytes(r, int(r.UintN(12)))}
	},
	"FrequencyList": func(r *rand.Rand) Descriptor {
		d := &FrequencyList{Header: Header{Tag: TagFrequencyList}, CodingType: uint8(r.UintN(4))}
		for i := uint(0); i < 1+r.UintN(4); i++ {
			d.Frequencies = append(d.Frequencies, r.Uint32())
		}
		return d
	},
	"FTAContentManagement": func(r *rand.Rand) Descriptor {
		return &FTAContentManagement{Header: Header{Tag: TagFTAContentManagement},
			UserDefined: r.UintN(2) == 1, DoNotScramble: r.UintN(2) == 1,
			ControlRemoteAccessOverInternet: uint8(r.UintN(4)), DoNotApplyRevocation: r.UintN(2) == 1}
	},
	"MultilingualNetworkName": func(r *rand.Rand) Descriptor {
		d := &MultilingualNetworkName{Header: Header{Tag: TagMultilingualNetworkName}}
		for i := uint(0); i < 1+r.UintN(3); i++ {
			d.Items = append(d.Items, MultilingualNetworkNameItem{
				Language: randLang(r), Name: randBytes(r, int(r.UintN(16)))})
		}
		return d
	},
	"MultilingualBouquetName": func(r *rand.Rand) Descriptor {
		d := &MultilingualBouquetName{Header: Header{Tag: TagMultilingualBouquetName}}
		for i := uint(0); i < 1+r.UintN(3); i++ {
			d.Items = append(d.Items, MultilingualBouquetNameItem{
				Language: randLang(r), Name: randBytes(r, int(r.UintN(16)))})
		}
		return d
	},
	"MultilingualComponent": func(r *rand.Rand) Descriptor {
		d := &MultilingualComponent{Header: Header{Tag: TagMultilingualComponent}, ComponentTag: uint8(r.UintN(256))}
		for i := uint(0); i < 1+r.UintN(3); i++ {
			d.Items = append(d.Items, MultilingualComponentItem{
				Language: randLang(r), Description: randBytes(r, int(r.UintN(16)))})
		}
		return d
	},
	"MultilingualServiceName": func(r *rand.Rand) Descriptor {
		d := &MultilingualServiceName{Header: Header{Tag: TagMultilingualServiceName}}
		for i := uint(0); i < 1+r.UintN(3); i++ {
			d.Items = append(d.Items, MultilingualServiceNameItem{
				Language: randLang(r), Provider: randBytes(r, int(r.UintN(12))), Name: randBytes(r, int(r.UintN(12)))})
		}
		return d
	},
	"AdaptationFieldData": func(r *rand.Rand) Descriptor {
		return &AdaptationFieldData{Header: Header{Tag: TagAdaptationFieldData}, Identifier: uint8(r.UintN(256))}
	},
	"AncillaryData": func(r *rand.Rand) Descriptor {
		return &AncillaryData{Header: Header{Tag: TagAncillaryData}, Identifier: uint8(r.UintN(256))}
	},
	"AnnouncementSupport": func(r *rand.Rand) Descriptor {
		d := &AnnouncementSupport{Header: Header{Tag: TagAnnouncementSupport}, SupportIndicator: uint16(r.UintN(1 << 16))}
		for i := uint(0); i < 1+r.UintN(4); i++ {
			d.Announcements = append(d.Announcements, AnnouncementSupportItem{
				AnnouncementType: uint8(r.UintN(16)), ReferenceType: uint8(r.UintN(8)),
				OriginalNetworkID: uint16(r.UintN(1 << 16)), TransportStreamID: uint16(r.UintN(1 << 16)),
				ServiceID: uint16(r.UintN(1 << 16)), ComponentTag: uint8(r.UintN(256))})
		}
		return d
	},
	"CellFrequencyLink": func(r *rand.Rand) Descriptor {
		d := &CellFrequencyLink{Header: Header{Tag: TagCellFrequencyLink}}
		for i := uint(0); i < 1+r.UintN(3); i++ {
			cell := CellFrequencyLinkCell{CellID: uint16(r.UintN(1 << 16)), Frequency: r.Uint32()}
			for j := uint(0); j < r.UintN(3); j++ {
				cell.Subcells = append(cell.Subcells, CellFrequencyLinkSubcell{
					CellIDExtension: uint8(r.UintN(256)), TransposerFrequency: r.Uint32()})
			}
			d.Cells = append(d.Cells, cell)
		}
		return d
	},
	"CellList": func(r *rand.Rand) Descriptor {
		d := &CellList{Header: Header{Tag: TagCellList}}
		for i := uint(0); i < 1+r.UintN(3); i++ {
			cell := CellListCell{CellID: uint16(r.UintN(1 << 16)),
				CellLatitude: uint16(r.UintN(1 << 16)), CellLongitude: uint16(r.UintN(1 << 16)),
				CellExtentOfLatitude: uint16(r.UintN(1 << 12)), CellExtentOfLongitude: uint16(r.UintN(1 << 12))}
			for j := uint(0); j < r.UintN(3); j++ {
				cell.Subcells = append(cell.Subcells, CellListSubcell{
					CellIDExtension: uint8(r.UintN(256)),
					SubcellLatitude: uint16(r.UintN(1 << 16)), SubcellLongitude: uint16(r.UintN(1 << 16)),
					SubcellExtentOfLatitude: uint16(r.UintN(1 << 12)), SubcellExtentOfLongitude: uint16(r.UintN(1 << 12))})
			}
			d.Cells = append(d.Cells, cell)
		}
		return d
	},
	"DataBroadcast": func(r *rand.Rand) Descriptor {
		return &DataBroadcast{Header: Header{Tag: TagDataBroadcast},
			DataBroadcastID: uint16(r.UintN(1 << 16)), ComponentTag: uint8(r.UintN(256)),
			Selector: randBytes(r, int(r.UintN(16))), Language: randLang(r), Text: randBytes(r, int(r.UintN(16)))}
	},
	"DataBroadcastID": func(r *rand.Rand) Descriptor {
		return &DataBroadcastID{Header: Header{Tag: TagDataBroadcastID},
			DataBroadcastID: uint16(r.UintN(1 << 16)), Selector: randBytes(r, int(r.UintN(16)))}
	},
	"ShortSmoothingBuffer": func(r *rand.Rand) Descriptor {
		return &ShortSmoothingBuffer{Header: Header{Tag: TagShortSmoothingBuffer},
			SBSize: uint8(r.UintN(4)), SBLeakRate: uint8(r.UintN(64)), Reserved: randBytes(r, int(r.UintN(4)))}
	},
	"PartialTransportStream": func(r *rand.Rand) Descriptor {
		return &PartialTransportStream{Header: Header{Tag: TagPartialTransportStream},
			PeakRate: uint32(r.UintN(1 << 22)), MinimumOverallSmoothingRate: uint32(r.UintN(1 << 22)),
			MaximumOverallSmoothingBuffer: uint16(r.UintN(1 << 14))}
	},
	"Telephone": func(r *rand.Rand) Descriptor {
		return &Telephone{Header: Header{Tag: TagTelephone},
			ForeignAvailability: r.UintN(2) == 1, ConnectionType: uint8(r.UintN(32)),
			CountryPrefix: randBytes(r, int(r.UintN(4))), InternationalAreaCode: randBytes(r, int(r.UintN(8))),
			OperatorCode: randBytes(r, int(r.UintN(4))), NationalAreaCode: randBytes(r, int(r.UintN(8))),
			CoreNumber: randBytes(r, int(r.UintN(16)))}
	},
	"ExtensionCIAncillaryData": func(r *rand.Rand) Descriptor {
		return &Extension{Header: Header{Tag: TagExtension}, Tag: TagExtensionCIAncillaryData,
			CIAncillaryData: &ExtensionCIAncillaryData{Data: randBytes(r, int(r.UintN(16)))}}
	},
	"ExtensionCP": func(r *rand.Rand) Descriptor {
		return &Extension{Header: Header{Tag: TagExtension}, Tag: TagExtensionCP,
			CP: &ExtensionCP{CPSystemID: uint16(r.UintN(1 << 16)), CPPID: uint16(r.UintN(1 << 13)),
				PrivateData: randBytes(r, int(r.UintN(8)))}}
	},
	"ExtensionCPIdentifier": func(r *rand.Rand) Descriptor {
		e := &ExtensionCPIdentifier{}
		for i := uint(0); i < 1+r.UintN(4); i++ {
			e.SystemIDs = append(e.SystemIDs, uint16(r.UintN(1<<16)))
		}
		return &Extension{Header: Header{Tag: TagExtension}, Tag: TagExtensionCPIdentifier, CPIdentifier: e}
	},
	"ExtensionC2DeliverySystem": func(r *rand.Rand) Descriptor {
		return &Extension{Header: Header{Tag: TagExtension}, Tag: TagExtensionC2DeliverySystem,
			C2DeliverySystem: &ExtensionC2DeliverySystem{
				PLPID: uint8(r.UintN(256)), DataSliceID: uint8(r.UintN(256)),
				C2SystemTuningFrequency: r.Uint32(), C2SystemTuningFrequencyType: uint8(r.UintN(4)),
				ActiveOFDMSymbolDuration: uint8(r.UintN(8)), GuardInterval: uint8(r.UintN(8))}}
	},
	"ExtensionC2BundleDeliverySystem": func(r *rand.Rand) Descriptor {
		e := &ExtensionC2BundleDeliverySystem{}
		for i := uint(0); i < 1+r.UintN(3); i++ {
			e.Entries = append(e.Entries, C2BundleEntry{
				PLPID: uint8(r.UintN(256)), DataSliceID: uint8(r.UintN(256)),
				C2SystemTuningFrequency: r.Uint32(), C2SystemTuningFrequencyType: uint8(r.UintN(4)),
				ActiveOFDMSymbolDuration: uint8(r.UintN(8)), GuardInterval: uint8(r.UintN(8)),
				MasterChannel: r.UintN(2) == 1})
		}
		return &Extension{Header: Header{Tag: TagExtension}, Tag: TagExtensionC2BundleDeliverySystem, C2BundleDeliverySystem: e}
	},
	"ExtensionSHDeliverySystem": func(r *rand.Rand) Descriptor {
		sh := &ExtensionSHDeliverySystem{DiversityMode: uint8(r.UintN(16))}
		for i := uint(0); i < 1+r.UintN(3); i++ {
			sh.Modulations = append(sh.Modulations, SHModulation{
				ModulationType: uint8(r.UintN(2)), InterleaverPresence: r.UintN(2) == 1, InterleaverType: r.UintN(2) == 1,
				Polarization: uint8(r.UintN(4)), RollOff: uint8(r.UintN(4)), ModulationMode: uint8(r.UintN(4)),
				SymbolRate: uint8(r.UintN(32)), Bandwidth: uint8(r.UintN(8)), Priority: r.UintN(2) == 1,
				ConstellationAndHierarchy: uint8(r.UintN(8)), GuardInterval: uint8(r.UintN(4)),
				TransmissionMode: uint8(r.UintN(4)), CommonFrequency: r.UintN(2) == 1, CodeRate: uint8(r.UintN(16)),
				CommonMultiplier: uint8(r.UintN(64)), NofLateTaps: uint8(r.UintN(64)), NofSlices: uint8(r.UintN(64)),
				SliceDistance: uint8(r.UintN(256)), NonLateIncrements: uint8(r.UintN(64))})
		}
		return &Extension{Header: Header{Tag: TagExtension}, Tag: TagExtensionSHDeliverySystem, SHDeliverySystem: sh}
	},
	"ExtensionMessage": func(r *rand.Rand) Descriptor {
		return &Extension{Header: Header{Tag: TagExtension}, Tag: TagExtensionMessage,
			Message: &ExtensionMessage{MessageID: uint8(r.UintN(256)), Language: randLang(r),
				Text: randBytes(r, int(r.UintN(20)))}}
	},
	"ExtensionServiceRelocated": func(r *rand.Rand) Descriptor {
		return &Extension{Header: Header{Tag: TagExtension}, Tag: TagExtensionServiceRelocated,
			ServiceRelocated: &ExtensionServiceRelocated{OldOriginalNetworkID: uint16(r.UintN(1 << 16)),
				OldTransportStreamID: uint16(r.UintN(1 << 16)), OldServiceID: uint16(r.UintN(1 << 16))}}
	},
	"ExtensionT2DeliverySystem": func(r *rand.Rand) Descriptor {
		t2 := &ExtensionT2DeliverySystem{PLPID: uint8(r.UintN(256)), T2SystemID: uint16(r.UintN(1 << 16)),
			HasExtension: r.UintN(2) == 1}
		if t2.HasExtension {
			t2.SISOMISO = uint8(r.UintN(4))
			t2.Bandwidth = uint8(r.UintN(16))
			t2.GuardInterval = uint8(r.UintN(8))
			t2.TransmissionMode = uint8(r.UintN(8))
			t2.OtherFrequencyFlag = r.UintN(2) == 1
			t2.TFSFlag = r.UintN(2) == 1
			for i := uint(0); i < 1+r.UintN(3); i++ {
				cell := T2Cell{CellID: uint16(r.UintN(1 << 16))}
				freqs := uint(1)
				if t2.TFSFlag {
					freqs = 1 + r.UintN(3)
				}
				for j := uint(0); j < freqs; j++ {
					cell.CentreFrequencies = append(cell.CentreFrequencies, r.Uint32())
				}
				for j := uint(0); j < r.UintN(3); j++ {
					cell.Subcells = append(cell.Subcells, T2Subcell{
						CellIDExtension: uint8(r.UintN(256)), TransposerFrequency: r.Uint32()})
				}
				t2.Cells = append(t2.Cells, cell)
			}
		}
		return &Extension{Header: Header{Tag: TagExtension}, Tag: TagExtensionT2DeliverySystem, T2DeliverySystem: t2}
	},
	"ExtensionImageIcon": func(r *rand.Rand) Descriptor {
		ii := &ExtensionImageIcon{LastDescriptorNumber: uint8(r.UintN(16)), IconID: uint8(r.UintN(8))}
		if r.UintN(2) == 1 {
			ii.DescriptorNumber = uint8(1 + r.UintN(15))
			ii.IconData = randBytes(r, int(r.UintN(20)))
		} else {
			ii.IconTransportMode = uint8(r.UintN(2))
			ii.PositionFlag = r.UintN(2) == 1
			ii.CoordinateSystem = uint8(r.UintN(8))
			ii.IconHorizontalOrigin = uint16(r.UintN(1 << 12))
			ii.IconVerticalOrigin = uint16(r.UintN(1 << 12))
			ii.IconType = randBytes(r, int(r.UintN(8)))
			ii.IconData = randBytes(r, int(r.UintN(20)))
		}
		return &Extension{Header: Header{Tag: TagExtension}, Tag: TagExtensionImageIcon, ImageIcon: ii}
	},
	"ExtensionTargetRegion": func(r *rand.Rand) Descriptor {
		tr := &ExtensionTargetRegion{CountryCode: randLang(r)}
		for i := uint(0); i < 1+r.UintN(3); i++ {
			tr.Regions = append(tr.Regions, TargetRegion{
				CountryCodeFlag: r.UintN(2) == 1, CountryCode: randLang(r), RegionDepth: uint8(r.UintN(4)),
				PrimaryRegionCode: uint8(r.UintN(256)), SecondaryRegionCode: uint8(r.UintN(256)),
				TertiaryRegionCode: uint16(r.UintN(1 << 16))})
		}
		return &Extension{Header: Header{Tag: TagExtension}, Tag: TagExtensionTargetRegion, TargetRegion: tr}
	},
	"ExtensionTargetRegionName": func(r *rand.Rand) Descriptor {
		tr := &ExtensionTargetRegionName{CountryCode: randLang(r), Language: randLang(r)}
		for i := uint(0); i < 1+r.UintN(3); i++ {
			tr.Regions = append(tr.Regions, TargetRegionName{
				RegionDepth: uint8(1 + r.UintN(3)), RegionName: randBytes(r, int(r.UintN(16))),
				PrimaryRegionCode: uint8(r.UintN(256)), SecondaryRegionCode: uint8(r.UintN(256)),
				TertiaryRegionCode: uint16(r.UintN(1 << 16))})
		}
		return &Extension{Header: Header{Tag: TagExtension}, Tag: TagExtensionTargetRegionName, TargetRegionName: tr}
	},
	"ExtensionT2MI": func(r *rand.Rand) Descriptor {
		return &Extension{Header: Header{Tag: TagExtension}, Tag: TagExtensionT2MI,
			T2MI: &ExtensionT2MI{T2MIStreamID: uint8(r.UintN(8)), NumT2MIStreamsMinusOne: uint8(r.UintN(8)),
				PCRISCRCommonClockFlag: r.UintN(2) == 1, Reserved: randBytes(r, int(r.UintN(4)))}}
	},
	"ExtensionURILinkage": func(r *rand.Rand) Descriptor {
		ul := &ExtensionURILinkage{URILinkageType: uint8(r.UintN(4)), URI: randBytes(r, int(r.UintN(16))),
			PrivateData: randBytes(r, int(r.UintN(8)))}
		if ul.URILinkageType == 0 || ul.URILinkageType == 1 {
			ul.MinPollingInterval = uint16(r.UintN(1 << 16))
		}
		return &Extension{Header: Header{Tag: TagExtension}, Tag: TagExtensionURILinkage, URILinkage: ul}
	},
	"ExtensionVideoDepthRange": func(r *rand.Rand) Descriptor {
		vd := &ExtensionVideoDepthRange{}
		for i := uint(0); i < 1+r.UintN(3); i++ {
			rng := VideoDepthRange{RangeType: uint8(r.UintN(4))}
			switch rng.RangeType {
			case 0:
				rng.VideoMaxDisparityHint = uint16(r.UintN(1 << 12))
				rng.VideoMinDisparityHint = uint16(r.UintN(1 << 12))
			case 1:
			default:
				rng.RangeSelector = randBytes(r, 1+int(r.UintN(8)))
			}
			vd.Ranges = append(vd.Ranges, rng)
		}
		return &Extension{Header: Header{Tag: TagExtension}, Tag: TagExtensionVideoDepthRange, VideoDepthRange: vd}
	},
	"ExtensionNetworkChangeNotify": func(r *rand.Rand) Descriptor {
		ncn := &ExtensionNetworkChangeNotify{}
		for i := uint(0); i < 1+r.UintN(2); i++ {
			cell := NetworkChangeCell{CellID: uint16(r.UintN(1 << 16))}
			for j := uint(0); j < 1+r.UintN(2); j++ {
				cell.Changes = append(cell.Changes, NetworkChange{
					NetworkChangeID: uint8(r.UintN(256)), NetworkChangeVersion: uint8(r.UintN(256)),
					StartTimeOfChange: r.Uint64N(1 << 40), ChangeDuration: uint32(r.UintN(1 << 24)),
					ReceiverCategory: uint8(r.UintN(8)), InvariantTSPresent: r.UintN(2) == 1,
					ChangeType: uint8(r.UintN(16)), MessageID: uint8(r.UintN(256)),
					InvariantTSTSID: uint16(r.UintN(1 << 16)), InvariantTSONID: uint16(r.UintN(1 << 16))})
			}
			ncn.Cells = append(ncn.Cells, cell)
		}
		return &Extension{Header: Header{Tag: TagExtension}, Tag: TagExtensionNetworkChangeNotify, NetworkChangeNotify: ncn}
	},
	"Mosaic": func(r *rand.Rand) Descriptor {
		d := &Mosaic{Header: Header{Tag: TagMosaic}, MosaicEntryPoint: r.UintN(2) == 1,
			NumberOfHorizontalElementaryCells: uint8(r.UintN(8)), NumberOfVerticalElementaryCells: uint8(r.UintN(8))}
		for i := uint(0); i < 1+r.UintN(3); i++ {
			cell := MosaicCell{
				LogicalCellID: uint8(r.UintN(64)), LogicalCellPresentationInfo: uint8(r.UintN(8)),
				CellLinkageInfo: uint8(r.UintN(5)),
				BouquetID:       uint16(r.UintN(1 << 16)), OriginalNetworkID: uint16(r.UintN(1 << 16)),
				TransportStreamID: uint16(r.UintN(1 << 16)), ServiceID: uint16(r.UintN(1 << 16)),
				EventID: uint16(r.UintN(1 << 16))}
			for j := uint(0); j < 1+r.UintN(3); j++ {
				cell.ElementaryCellIDs = append(cell.ElementaryCellIDs, uint8(r.UintN(64)))
			}
			d.Cells = append(d.Cells, cell)
		}
		return d
	},
}

func TestRoundtripDescriptors(t *testing.T) {
	for name, gen := range roundtripGenerators {
		t.Run(name, func(t *testing.T) {
			r := rand.New(rand.NewPCG(5, 6))
			for i := 0; i < 200; i++ {
				d := gen(r)
				b1 := AppendWithLength(nil, []Descriptor{d})

				ds, n, err := Parse(b1)
				require.NoError(t, err, "iteration %d", i)
				require.Equal(t, len(b1), n, "iteration %d", i)
				require.Len(t, ds, 1, "iteration %d", i)

				b2 := AppendWithLength(nil, ds)
				assert.Equal(t, b1, b2, "iteration %d", i)
			}
		})
	}
}
