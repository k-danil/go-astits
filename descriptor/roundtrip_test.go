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
