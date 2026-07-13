package descriptor

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v2/descriptor/ext"
	"github.com/k-danil/go-astits/v2/internal/bitstest"
)

var (
	dvbMinutesDuration      = time.Hour + 45*time.Minute
	dvbMinutesDurationBytes = []byte{0x1, 0x45} // 0145
	dvbTime, _              = time.Parse("2006-01-02 15:04:05", "1993-10-13 12:45:00")
	dvbTimeBytes            = []byte{0xc0, 0x79, 0x12, 0x45, 0x0} // C079124500
)

type descriptorTest struct {
	name      string
	bytesFunc func(w *bitstest.Writer)
	desc      Descriptor
}

var descriptorTestTable = []descriptorTest{
	{
		"AC3",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(TagAC3))  // Tag
			_ = w.Write(uint8(9))       // Length
			_ = w.Write("1")            // Component type flag
			_ = w.Write("1")            // BSID flag
			_ = w.Write("1")            // MainID flag
			_ = w.Write("1")            // ASVC flag
			_ = w.Write("1111")         // Reserved flags
			_ = w.Write(uint8(1))       // Component type
			_ = w.Write(uint8(2))       // BSID
			_ = w.Write(uint8(3))       // MainID
			_ = w.Write(uint8(4))       // ASVC
			_ = w.Write([]byte("info")) // Additional info
		},
		&AC3{
			Header: Header{
				Tag:    TagAC3,
				Length: 9,
			},
			AdditionalInfo:   []byte("info"),
			ASVC:             uint8(4),
			BSID:             uint8(2),
			ComponentType:    uint8(1),
			HasASVC:          true,
			HasBSID:          true,
			HasComponentType: true,
			HasMainID:        true,
			MainID:           uint8(3),
		}},
	{
		"ISO639LanguageAndAudioType",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(TagISO639LanguageAndAudioType)) // Tag
			_ = w.Write(uint8(8))                             // Length
			_ = w.Write([]byte("eng"))                        // Language #1
			_ = w.Write(uint8(AudioTypeCleanEffects))         // Audio type #1
			_ = w.Write([]byte("rus"))                        // Language #2
			_ = w.Write(uint8(AudioTypeHearingImpaired))      // Audio type #2
		},
		&ISO639LanguageAndAudioType{
			Header: Header{
				Tag:    TagISO639LanguageAndAudioType,
				Length: 8,
			},
			Items: []ISO639Item{
				{Language: [3]byte{'e', 'n', 'g'}, Type: AudioTypeCleanEffects},
				{Language: [3]byte{'r', 'u', 's'}, Type: AudioTypeHearingImpaired},
			},
		}},
	{
		"MaximumBitrate",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(TagMaximumBitrate))   // Tag
			_ = w.Write(uint8(3))                   // Length
			_ = w.Write("110000000000000000000001") // Maximum bitrate
		},
		&MaximumBitrate{
			Header: Header{
				Tag:    TagMaximumBitrate,
				Length: 3,
			},
			Bitrate: uint32(50)}},
	{
		"NetworkName",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(TagNetworkName)) // Tag
			_ = w.Write(uint8(4))              // Length
			_ = w.Write([]byte("name"))        // Name
		},
		&NetworkName{
			Header: Header{
				Tag:    TagNetworkName,
				Length: 4,
			},
			Name: []byte("name")}},
	{
		"Service",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(TagService))                          // Tag
			_ = w.Write(uint8(18))                                  // Length
			_ = w.Write(uint8(ServiceTypeDigitalTelevisionService)) // Type
			_ = w.Write(uint8(8))                                   // Provider name length
			_ = w.Write([]byte("provider"))                         // Provider name
			_ = w.Write(uint8(7))                                   // Service name length
			_ = w.Write([]byte("service"))                          // Service name
		},
		&Service{
			Header: Header{
				Tag:    TagService,
				Length: 18,
			},
			Name:     []byte("service"),
			Provider: []byte("provider"),
			Type:     ServiceTypeDigitalTelevisionService,
		}},
	{
		"ShortEvent",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(TagShortEvent)) // Tag
			_ = w.Write(uint8(14))            // Length
			_ = w.Write([]byte("eng"))        // Language code
			_ = w.Write(uint8(5))             // Event name length
			_ = w.Write([]byte("event"))      // Event name
			_ = w.Write(uint8(4))             // Text length
			_ = w.Write([]byte("text"))
		},
		&ShortEvent{
			Header: Header{
				Tag:    TagShortEvent,
				Length: 14,
			},
			EventName: []byte("event"),
			Language:  [3]byte{0x65, 0x6e, 0x67}, // eng
			Text:      []byte("text"),
		}},
	{
		"StreamIdentifier",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(TagStreamIdentifier)) // Tag
			_ = w.Write(uint8(1))                   // Length
			_ = w.Write(uint8(2))                   // Component tag
		},
		&StreamIdentifier{
			Header: Header{
				Tag:    TagStreamIdentifier,
				Length: 1,
			},
			ComponentTag: 0x2}},
	{
		"Subtitling",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(TagSubtitling)) // Tag
			_ = w.Write(uint8(16))            // Length
			_ = w.Write([]byte("lg1"))        // Item #1 language
			_ = w.Write(uint8(1))             // Item #1 type
			_ = w.Write(uint16(2))            // Item #1 composition page
			_ = w.Write(uint16(3))            // Item #1 ancillary page
			_ = w.Write([]byte("lg2"))        // Item #2 language
			_ = w.Write(uint8(4))             // Item #2 type
			_ = w.Write(uint16(5))            // Item #2 composition page
			_ = w.Write(uint16(6))            // Item #2 ancillary page
		},
		&Subtitling{
			Header: Header{
				Tag:    TagSubtitling,
				Length: 16,
			},
			Items: []SubtitlingItem{
				{
					AncillaryPageID:   3,
					CompositionPageID: 2,
					Language:          [3]byte{0x6c, 0x67, 0x31}, // lg1
					Type:              1,
				},
				{
					AncillaryPageID:   6,
					CompositionPageID: 5,
					Language:          [3]byte{0x6c, 0x67, 0x32}, // lg2
					Type:              4,
				},
			}}},
	{
		"Teletext",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(TagTeletext)) // Tag
			_ = w.Write(uint8(10))          // Length
			_ = w.Write([]byte("lg1"))      // Item #1 language
			_ = w.Write("00001")            // Item #1 type
			_ = w.Write("010")              // Item #1 magazine
			_ = w.Write("00010010")         // Item #1 page number
			_ = w.Write([]byte("lg2"))      // Item #2 language
			_ = w.Write("00011")            // Item #2 type
			_ = w.Write("100")              // Item #2 magazine
			_ = w.Write("00100011")         // Item #2 page number
		},
		&Teletext{
			Header: Header{
				Tag:    TagTeletext,
				Length: 10,
			},
			Items: []TeletextItem{
				{
					Language: [3]byte{0x6c, 0x67, 0x31}, // lg1
					Magazine: uint8(2),
					Page:     uint8(0x12),
					Type:     TeletextType(1),
				},
				{
					Language: [3]byte{0x6c, 0x67, 0x32}, // lg2
					Magazine: uint8(4),
					Page:     uint8(0x23),
					Type:     TeletextType(3),
				},
			}}},
	{
		"ExtendedEvent",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(TagExtendedEvent)) // Tag
			_ = w.Write(uint8(30))               // Length
			_ = w.Write("0001")                  // Number
			_ = w.Write("0010")                  // Last descriptor number
			_ = w.Write([]byte("lan"))           // ISO 639 language code
			_ = w.Write(uint8(20))               // Length of items
			_ = w.Write(uint8(11))               // Item #1 description length
			_ = w.Write([]byte("description"))   // Item #1 description
			_ = w.Write(uint8(7))                // Item #1 content length
			_ = w.Write([]byte("content"))       // Item #1 content
			_ = w.Write(uint8(4))                // Text length
			_ = w.Write([]byte("text"))          // Text
		},
		&ExtendedEvent{
			Header: Header{
				Tag:    TagExtendedEvent,
				Length: 30,
			},
			ISO639LanguageCode: [3]byte{0x6c, 0x61, 0x6e}, // lan
			Items: []ExtendedEventItem{{
				Content:     []byte("content"),
				Description: []byte("description"),
			}},
			LastDescriptorNumber: 0x2,
			Number:               0x1,
			Text:                 []byte("text"),
		}},
	{
		"EnhancedAC3",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(TagEnhancedAC3)) // Tag
			_ = w.Write(uint8(12))             // Length
			_ = w.Write("1")                   // Component type flag
			_ = w.Write("1")                   // BSID flag
			_ = w.Write("1")                   // MainID flag
			_ = w.Write("1")                   // ASVC flag
			_ = w.Write("1")                   // Mix info exists
			_ = w.Write("1")                   // SubStream1 flag
			_ = w.Write("1")                   // SubStream2 flag
			_ = w.Write("1")                   // SubStream3 flag
			_ = w.Write(uint8(1))              // Component type
			_ = w.Write(uint8(2))              // BSID
			_ = w.Write(uint8(3))              // MainID
			_ = w.Write(uint8(4))              // ASVC
			_ = w.Write(uint8(5))              // SubStream1
			_ = w.Write(uint8(6))              // SubStream2
			_ = w.Write(uint8(7))              // SubStream3
			_ = w.Write([]byte("info"))        // Additional info
		},
		&EnhancedAC3{
			Header: Header{
				Tag:    TagEnhancedAC3,
				Length: 12,
			},
			AdditionalInfo:   []byte("info"),
			ASVC:             uint8(4),
			BSID:             uint8(2),
			ComponentType:    uint8(1),
			HasASVC:          true,
			HasBSID:          true,
			HasComponentType: true,
			HasMainID:        true,
			HasSubStream1:    true,
			HasSubStream2:    true,
			HasSubStream3:    true,
			MainID:           uint8(3),
			MixInfoExists:    true,
			SubStream1:       5,
			SubStream2:       6,
			SubStream3:       7,
		}},
	{
		"Extension",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(TagExtension))              // Tag
			_ = w.Write(uint8(12))                        // Length
			_ = w.Write(uint8(ext.TagSupplementaryAudio)) // Extension tag
			_ = w.Write("1")                              // Mix type
			_ = w.Write("10101")                          // Editorial classification
			_ = w.Write("1")                              // Reserved
			_ = w.Write("1")                              // Language code flag
			_ = w.Write([]byte("lan"))                    // Language code
			_ = w.Write([]byte("private"))                // Private data
		},
		&Extension{
			Header: Header{
				Tag:    TagExtension,
				Length: 12,
			},
			Body: &ext.SupplementaryAudio{
				EditorialClassification: 21,
				HasLanguageCode:         true,
				LanguageCode:            [3]byte{0x6c, 0x61, 0x6e}, // lan
				MixType:                 true,
				PrivateData:             []byte("private"),
			},
		}},
	{
		"Component",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(TagComponent)) // Tag
			_ = w.Write(uint8(10))           // Length
			_ = w.Write("1010")              // Stream content ext
			_ = w.Write("0101")              // Stream content
			_ = w.Write(uint8(1))            // Component type
			_ = w.Write(uint8(2))            // Component tag
			_ = w.Write([]byte("lan"))       // ISO639 language code
			_ = w.Write([]byte("text"))      // Text
		},
		&Component{
			Header: Header{
				Tag:    TagComponent,
				Length: 10,
			},
			ComponentTag:       2,
			ComponentType:      1,
			ISO639LanguageCode: [3]byte{0x6c, 0x61, 0x6e}, // lan
			StreamContentExt:   10,
			StreamContent:      5,
			Text:               []byte("text"),
		}},
	{
		"Content",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(TagContent)) // Tag
			_ = w.Write(uint8(2))          // Length
			_ = w.Write("0001")            // Item #1 content nibble level 1
			_ = w.Write("0010")            // Item #1 content nibble level 2
			_ = w.Write(uint8(3))          // Item #1 user byte
		},
		&Content{
			Header: Header{
				Tag:    TagContent,
				Length: 2,
			},
			Items: []ContentItem{{
				ContentNibbleLevel1: 1,
				ContentNibbleLevel2: 2,
				UserByte:            3,
			}}}},
	{
		"ParentalRating",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(TagParentalRating)) // Tag
			_ = w.Write(uint8(4))                 // Length
			_ = w.Write([]byte("cou"))            // Item #1 country code
			_ = w.Write(uint8(2))                 // Item #1 rating
		},
		&ParentalRating{
			Header: Header{
				Tag:    TagParentalRating,
				Length: 4,
			},
			Items: []ParentalRatingItem{{
				CountryCode: [3]byte{0x63, 0x6f, 0x75}, // cou
				Rating:      2,
			}}}},
	{
		"LocalTimeOffset",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(TagLocalTimeOffset)) // Tag
			_ = w.Write(uint8(13))                 // Length
			_ = w.Write([]byte("cou"))             // Country code
			_ = w.Write("101010")                  // Country region ID
			_ = w.Write("1")                       // Reserved
			_ = w.Write("1")                       // Local time offset polarity
			_ = w.Write(dvbMinutesDurationBytes)   // Local time offset
			_ = w.Write(dvbTimeBytes)              // Time of change
			_ = w.Write(dvbMinutesDurationBytes)   // Next time offset
		},
		&LocalTimeOffset{
			Header: Header{
				Tag:    TagLocalTimeOffset,
				Length: 13,
			},
			Items: []LocalTimeOffsetItem{{
				CountryCode:             [3]byte{0x63, 0x6f, 0x75}, // cou
				CountryRegionID:         42,
				LocalTimeOffset:         dvbMinutesDuration,
				LocalTimeOffsetPolarity: true,
				NextTimeOffset:          dvbMinutesDuration,
				TimeOfChange:            dvbTime,
			}}}},
	{
		"VBIData",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(TagVBIData))                  // Tag
			_ = w.Write(uint8(3))                           // Length
			_ = w.Write(uint8(VBIDataServiceIDEBUTeletext)) // Service #1 id
			_ = w.Write(uint8(1))                           // Service #1 descriptor length
			_ = w.Write("11")                               // Service #1 descriptor reserved
			_ = w.Write("1")                                // Service #1 descriptor field polarity
			_ = w.Write("10101")                            // Service #1 descriptor line offset
		},
		&VBIData{
			Header: Header{
				Tag:    TagVBIData,
				Length: 3,
			},
			Services: []VBIDataService{{
				DataServiceID: VBIDataServiceIDEBUTeletext,
				Descriptors: []VBIDataDescriptor{{
					FieldParity: true,
					LineOffset:  21,
				}},
			}}}},
	{
		"VBITeletext",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(TagVBITeletext)) // Tag
			_ = w.Write(uint8(5))              // Length
			_ = w.Write([]byte("lan"))         // Item #1 language
			_ = w.Write("00001")               // Item #1 type
			_ = w.Write("010")                 // Item #1 magazine
			_ = w.Write("00010010")            // Item #1 page number
		},
		&Teletext{
			Header: Header{
				Tag:    TagVBITeletext,
				Length: 5,
			},
			Items: []TeletextItem{{
				Language: [3]byte{0x6c, 0x61, 0x6e}, // lan
				Magazine: uint8(2),
				Page:     uint8(0x12),
				Type:     TeletextType(1),
			}}}},
	{
		"AVCVideo",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(TagAVCVideo)) // Tag
			_ = w.Write(uint8(4))           // Length
			_ = w.Write(uint8(1))           // Profile idc
			_ = w.Write("1")                // Constraint set0 flag
			_ = w.Write("1")                // Constraint set1 flag
			_ = w.Write("1")                // Constraint set2 flag
			_ = w.Write("1")                // Constraint set3 flag
			_ = w.Write("0")                // Constraint set4 flag
			_ = w.Write("1")                // Constraint set5 flag
			_ = w.Write("01")               // Compatible flags (2 bits)
			_ = w.Write(uint8(2))           // Level idc
			_ = w.Write("1")                // AVC still present
			_ = w.Write("1")                // AVC 24 hour picture flag
			_ = w.Write("1")                // Frame packing SEI not present flag
			_ = w.Write("11111")            // Reserved (5 bits)
		},
		&AVCVideo{
			Header: Header{
				Tag:    TagAVCVideo,
				Length: 4,
			},
			AVC24HourPictureFlag:          true,
			AVCStillPresent:               true,
			FramePackingSEINotPresentFlag: true,
			CompatibleFlags:               1,
			ConstraintSet0Flag:            true,
			ConstraintSet1Flag:            true,
			ConstraintSet2Flag:            true,
			ConstraintSet3Flag:            true,
			ConstraintSet5Flag:            true,
			LevelIDC:                      2,
			ProfileIDC:                    1,
		}},
	{
		"PrivateDataSpecifier",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(TagPrivateDataSpecifier)) // Tag
			_ = w.Write(uint8(4))                       // Length
			_ = w.Write(uint32(128))                    // Private data specifier
		},
		&PrivateDataSpecifier{
			Header: Header{
				Tag:    TagPrivateDataSpecifier,
				Length: 4,
			},
			Specifier: 128,
		}},
	{
		"DataStreamAlignment",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(TagDataStreamAlignment)) // Tag
			_ = w.Write(uint8(1))                      // Length
			_ = w.Write(uint8(2))                      // Type
		},
		&DataStreamAlignment{
			Header: Header{
				Tag:    TagDataStreamAlignment,
				Length: 1,
			},
			Type: 2,
		}},
	{
		"PrivateDataIndicator",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(TagPrivateDataIndicator)) // Tag
			_ = w.Write(uint8(4))                       // Length
			_ = w.Write(uint32(127))                    // Private data indicator
		},
		&PrivateDataIndicator{
			Header: Header{
				Tag:    TagPrivateDataIndicator,
				Length: 4,
			},
			Indicator: 127,
		}},
	{
		"UserDefined",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(0x80))    // Tag
			_ = w.Write(uint8(4))       // Length
			_ = w.Write([]byte("test")) // User defined
		},
		&UserDefined{
			Header: Header{
				Tag:    0x80,
				Length: 4,
			},
			Data: []byte("test")}},
	{
		"Registration",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(TagRegistration)) // Tag
			_ = w.Write(uint8(8))               // Length
			_ = w.Write(uint32(1))              // Format identifier
			_ = w.Write([]byte("test"))         // Additional identification info
		},
		&Registration{
			Header: Header{
				Tag:    TagRegistration,
				Length: 8,
			},
			AdditionalIdentificationInfo: []byte("test"),
			FormatIdentifier:             uint32(1),
		}},
	{
		"Unknown",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(0x1))     // Tag
			_ = w.Write(uint8(4))       // Length
			_ = w.Write([]byte("test")) // Content
		},
		&Unknown{
			Header: Header{
				Tag:    0x1,
				Length: 4,
			},
			Content: []byte("test"),
		}},
	{
		"Extension",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(TagExtension)) // Tag
			_ = w.Write(uint8(5))            // Length
			_ = w.Write(uint8(0x12))         // Extension tag (reserved → Unknown)
			_ = w.Write([]byte("test"))      // Content
		},
		&Extension{
			Header: Header{
				Tag:    TagExtension,
				Length: 5,
			},
			Body: &ext.Unknown{ExtTag: 0x12, Data: []byte{'t', 'e', 's', 't'}},
		}},
}

func TestParseDescriptorOneByOne(t *testing.T) {
	for _, tc := range descriptorTestTable {
		t.Run(tc.name, func(t *testing.T) {
			// idea is following:
			// 1. get descriptor bytes and update its length
			// 2. parse bytes and get a Descriptor instance
			// 3. compare expected descriptor value and actual
			buf := bytes.Buffer{}
			buf.Write([]byte{0x00, 0x00}) // reserve two bytes for length
			w := bitstest.NewWriter(&buf)
			tc.bytesFunc(w)
			descLen := uint16(buf.Len() - 2)
			descBytes := buf.Bytes()
			descBytes[0] = byte(descLen >> 8)
			descBytes[1] = byte(descLen & 0xff)

			ds, _, err := Parse(descBytes)
			assert.NoError(t, err)
			assert.Equal(t, tc.desc, ds[0])
		})
	}
}

func TestParseDescriptorAll(t *testing.T) {
	buf := bytes.Buffer{}
	buf.Write([]byte{0x00, 0x00}) // reserve two bytes for length
	w := bitstest.NewWriter(&buf)

	for _, tc := range descriptorTestTable {
		tc.bytesFunc(w)
	}

	descLen := uint16(buf.Len() - 2)
	descBytes := buf.Bytes()
	descBytes[0] = byte(descLen >> 8)
	descBytes[1] = byte(descLen & 0xff)

	ds, _, err := Parse(descBytes)
	assert.NoError(t, err)

	for i, tc := range descriptorTestTable {
		assert.Equal(t, tc.desc, ds[i])
	}
}

func TestWriteDescriptorOneByOne(t *testing.T) {
	for _, tc := range descriptorTestTable {
		t.Run(tc.name, func(t *testing.T) {
			bufExpected := bytes.Buffer{}
			wExpected := bitstest.NewWriter(&bufExpected)
			tc.bytesFunc(wExpected)

			actual := tc.desc.Append(nil)
			assert.Equal(t, 2+tc.desc.CalcLength(), len(actual))
			assert.Equal(t, bufExpected.Bytes(), actual)
		})
	}
}

func TestWriteDescriptorAll(t *testing.T) {
	bufExpected := bytes.Buffer{}
	bufExpected.Write([]byte{0x00, 0x00}) // reserve two bytes for length
	wExpected := bitstest.NewWriter(&bufExpected)

	var dss []Descriptor

	for _, tc := range descriptorTestTable {
		tc.bytesFunc(wExpected)
		tcc := tc
		dss = append(dss, tcc.desc)
	}

	descLen := uint16(bufExpected.Len() - 2)
	descBytes := bufExpected.Bytes()
	descBytes[0] = byte(descLen>>8) | 0b11110000 // program_info_length is preceded by 4 reserved bits
	descBytes[1] = byte(descLen & 0xff)

	actual := AppendWithLength(nil, dss)
	assert.Equal(t, bufExpected.Len(), len(actual))
	assert.Equal(t, bufExpected.Bytes(), actual)
}

func BenchmarkWriteDescriptor(b *testing.B) {
	dst := make([]byte, 0, 1024)

	for _, bm := range descriptorTestTable {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				dst = bm.desc.Append(dst[:0])
			}
		})
	}
}

func BenchmarkParseDescriptor(b *testing.B) {
	bss := make([][]byte, len(descriptorTestTable))

	for ti, tc := range descriptorTestTable {
		buf := bytes.Buffer{}
		buf.Write([]byte{0x00, 0x00}) // reserve two bytes for length
		w := bitstest.NewWriter(&buf)
		tc.bytesFunc(w)
		descLen := uint16(buf.Len() - 2)
		descBytes := buf.Bytes()
		descBytes[0] = byte(descLen >> 8)
		descBytes[1] = byte(descLen & 0xff)
		bss[ti] = descBytes
	}

	for ti, tc := range descriptorTestTable {
		bs := bss[ti]
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _, _ = Parse(bs)
			}
		})
	}
}

// Parsed descriptors must own their payloads: no field may keep a view into
// the input buffer, it goes back to a pool right after parsing.
func TestParseOwnsInput(t *testing.T) {
	buf := bytes.Buffer{}
	buf.Write([]byte{0x00, 0x00})
	w := bitstest.NewWriter(&buf)
	for _, tc := range descriptorTestTable {
		tc.bytesFunc(w)
	}
	src := buf.Bytes()
	l := len(src) - 2
	src[0] = byte(l>>8) | 0xf0
	src[1] = byte(l)

	pristine := append([]byte(nil), src...)
	reference, _, err := Parse(pristine)
	require.NoError(t, err)

	parsed, _, err := Parse(src)
	require.NoError(t, err)
	for i := range src {
		src[i] = 0xa5
	}

	assert.Equal(t, reference, parsed)
}
