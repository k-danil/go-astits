package psi

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/k-danil/go-astits/v3/descriptor"
	"github.com/k-danil/go-astits/v3/internal/bytesiter"
	"github.com/k-danil/go-astits/v3/internal/util"
	"github.com/k-danil/go-astits/v3/pes"
)

type StreamType uint8

const (
	StreamTypeMPEG1Video                 StreamType = 0x01
	StreamTypeMPEG2Video                 StreamType = 0x02
	StreamTypeMPEG1Audio                 StreamType = 0x03
	StreamTypeMPEG2HalvedSampleRateAudio StreamType = 0x04
	StreamTypeMPEG2Audio                 StreamType = 0x04
	StreamTypePrivateSection             StreamType = 0x05
	StreamTypePrivateData                StreamType = 0x06
	StreamTypeMPEG2PacketizedData        StreamType = 0x06
	StreamTypeADTS                       StreamType = 0x0F
	StreamTypeAACAudio                   StreamType = 0x0f
	StreamTypeMPEG4Video                 StreamType = 0x10
	StreamTypeAACLATMAudio               StreamType = 0x11
	StreamTypeMetadata                   StreamType = 0x15
	StreamTypeH264Video                  StreamType = 0x1B
	StreamTypeMPEG4Audio                 StreamType = 0x1C
	StreamTypeAuxiliaryVideo             StreamType = 0x1E
	StreamTypeSVCVideo                   StreamType = 0x1F
	StreamTypeMVCVideo                   StreamType = 0x20
	StreamTypeJPEG2000Video              StreamType = 0x21
	StreamTypeMPEG2AdditionalViewVideo   StreamType = 0x22
	StreamTypeH264AdditionalViewVideo    StreamType = 0x23
	StreamTypeH265Video                  StreamType = 0x24
	StreamTypeHEVCVideo                  StreamType = 0x24
	StreamTypeHEVCTemporalVideo          StreamType = 0x25
	StreamTypeMVCDVideo                  StreamType = 0x26
	StreamTypeHEVCEnhancementG           StreamType = 0x28
	StreamTypeHEVCTemporalEnhancementG   StreamType = 0x29
	StreamTypeHEVCEnhancementH           StreamType = 0x2A
	StreamTypeHEVCTemporalEnhancementH   StreamType = 0x2B
	StreamTypeMPEGHAudioMain             StreamType = 0x2D
	StreamTypeMPEGHAudioAuxiliary        StreamType = 0x2E
	StreamTypeJPEGXSVideo                StreamType = 0x32
	StreamTypeVVCVideo                   StreamType = 0x33
	StreamTypeVVCTemporalVideo           StreamType = 0x34
	StreamTypeEVCVideo                   StreamType = 0x35
	StreamTypeLCEVCVideo                 StreamType = 0x36
	StreamTypeCAVSVideo                  StreamType = 0x42
	StreamTypeVC1Video                   StreamType = 0xea
	StreamTypeDIRACVideo                 StreamType = 0xd1
	StreamTypeAC3Audio                   StreamType = 0x81
	StreamTypeDTSAudio                   StreamType = 0x82
	StreamTypeTRUEHDAudio                StreamType = 0x83
	StreamTypeSCTE35                     StreamType = 0x86
	StreamTypeEAC3Audio                  StreamType = 0x87
)

type PMT struct {
	ElementaryStreams  []ElementaryStream      `json:"_elementary_streams"`
	ProgramDescriptors []descriptor.Descriptor `json:"_program_descriptors"`
	ProgramNumber      uint16                  `json:"program_number"`
	PCRPID             uint16                  `json:"PCR_PID"`
}

type ElementaryStream struct {
	ElementaryStreamDescriptors []descriptor.Descriptor `json:"_elementary_stream_descriptors"`
	ElementaryPID               uint16                  `json:"elementary_PID"`
	StreamType                  StreamType              `json:"stream_type"`
}

func parsePMTSection(i *bytesiter.Iterator, offsetSectionsEnd int, tableIDExtension uint16) (d *PMT, err error) {
	d = &PMT{ProgramNumber: tableIDExtension}

	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d.PCRPID = binary.BigEndian.Uint16(bs) & 0x1fff

	var dn int
	if d.ProgramDescriptors, dn, err = descriptor.Parse(i.Bytes()); err != nil {
		err = fmt.Errorf("astits: parsing descriptors failed: %w", err)
		return
	}
	i.Skip(dn)

	for i.Offset() < offsetSectionsEnd {
		e := ElementaryStream{}

		var b byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		e.StreamType = StreamType(b)

		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}

		e.ElementaryPID = binary.BigEndian.Uint16(bs) & 0x1fff

		if e.ElementaryStreamDescriptors, dn, err = descriptor.Parse(i.Bytes()); err != nil {
			err = fmt.Errorf("astits: parsing descriptors failed: %w", err)
			return
		}
		i.Skip(dn)

		d.ElementaryStreams = append(d.ElementaryStreams, e)
	}
	return
}

func (d *PMT) CalcSectionLength() int {
	ret := 4 + descriptor.CalcLength(d.ProgramDescriptors)

	for _, es := range d.ElementaryStreams {
		ret += 5 + descriptor.CalcLength(es.ElementaryStreamDescriptors)
	}

	return ret
}

func (d *PMT) appendSection(dst []byte) []byte {

	dst = append(dst, 0xe0|byte(d.PCRPID>>8)&0x1f, byte(d.PCRPID))
	dst = descriptor.AppendWithLength(dst, d.ProgramDescriptors)

	for _, es := range d.ElementaryStreams {
		dst = append(dst, uint8(es.StreamType), 0xe0|byte(es.ElementaryPID>>8)&0x1f, byte(es.ElementaryPID))
		dst = descriptor.AppendWithLength(dst, es.ElementaryStreamDescriptors)
	}

	return dst
}

func (t StreamType) IsVideo() bool {
	switch t {
	case StreamTypeMPEG1Video,
		StreamTypeMPEG2Video,
		StreamTypeMPEG4Video,
		StreamTypeH264Video,
		StreamTypeAuxiliaryVideo,
		StreamTypeSVCVideo,
		StreamTypeMVCVideo,
		StreamTypeJPEG2000Video,
		StreamTypeMPEG2AdditionalViewVideo,
		StreamTypeH264AdditionalViewVideo,
		StreamTypeH265Video,
		StreamTypeHEVCTemporalVideo,
		StreamTypeMVCDVideo,
		StreamTypeHEVCEnhancementG,
		StreamTypeHEVCTemporalEnhancementG,
		StreamTypeHEVCEnhancementH,
		StreamTypeHEVCTemporalEnhancementH,
		StreamTypeJPEGXSVideo,
		StreamTypeVVCVideo,
		StreamTypeVVCTemporalVideo,
		StreamTypeEVCVideo,
		StreamTypeLCEVCVideo,
		StreamTypeCAVSVideo,
		StreamTypeVC1Video,
		StreamTypeDIRACVideo:
		return true
	}
	return false
}

func (t StreamType) IsAudio() bool {
	switch t {
	case StreamTypeMPEG1Audio,
		StreamTypeMPEG2Audio,
		StreamTypeAACAudio,
		StreamTypeAACLATMAudio,
		StreamTypeMPEG4Audio,
		StreamTypeMPEGHAudioMain,
		StreamTypeMPEGHAudioAuxiliary,
		StreamTypeAC3Audio,
		StreamTypeDTSAudio,
		StreamTypeTRUEHDAudio,
		StreamTypeEAC3Audio:
		return true
	}
	return false
}

var streamTypeNames = map[StreamType]string{
	StreamTypeMPEG1Video:               "MPEG-1 video",
	StreamTypeMPEG2Video:               "MPEG-2 video",
	StreamTypeMPEG1Audio:               "MPEG-1 audio",
	StreamTypeMPEG2Audio:               "MPEG-2 audio",
	StreamTypePrivateSection:           "private_sections",
	StreamTypePrivateData:              "PES private data",
	StreamTypeAACAudio:                 "MPEG-2 AAC (ADTS)",
	StreamTypeMPEG4Video:               "MPEG-4 video",
	StreamTypeAACLATMAudio:             "MPEG-4 AAC (LATM)",
	StreamTypeMetadata:                 "metadata in PES",
	StreamTypeH264Video:                "AVC video",
	StreamTypeMPEG4Audio:               "MPEG-4 audio",
	StreamTypeAuxiliaryVideo:           "auxiliary video",
	StreamTypeSVCVideo:                 "SVC video",
	StreamTypeMVCVideo:                 "MVC video",
	StreamTypeJPEG2000Video:            "JPEG 2000 video",
	StreamTypeMPEG2AdditionalViewVideo: "MPEG-2 additional view video",
	StreamTypeH264AdditionalViewVideo:  "AVC additional view video",
	StreamTypeH265Video:                "HEVC video",
	StreamTypeHEVCTemporalVideo:        "HEVC temporal video",
	StreamTypeMVCDVideo:                "MVCD video",
	StreamTypeHEVCEnhancementG:         "HEVC enhancement (Annex G)",
	StreamTypeHEVCTemporalEnhancementG: "HEVC temporal enhancement (Annex G)",
	StreamTypeHEVCEnhancementH:         "HEVC enhancement (Annex H)",
	StreamTypeHEVCTemporalEnhancementH: "HEVC temporal enhancement (Annex H)",
	StreamTypeMPEGHAudioMain:           "MPEG-H audio (MHAS main)",
	StreamTypeMPEGHAudioAuxiliary:      "MPEG-H audio (MHAS auxiliary)",
	StreamTypeJPEGXSVideo:              "JPEG XS video",
	StreamTypeVVCVideo:                 "VVC video",
	StreamTypeVVCTemporalVideo:         "VVC temporal video",
	StreamTypeEVCVideo:                 "EVC video",
	StreamTypeLCEVCVideo:               "LCEVC video",
	StreamTypeCAVSVideo:                "CAVS video",
	StreamTypeVC1Video:                 "VC-1 video",
	StreamTypeDIRACVideo:               "Dirac video",
	StreamTypeAC3Audio:                 "AC-3 audio (ATSC)",
	StreamTypeDTSAudio:                 "DTS audio",
	StreamTypeTRUEHDAudio:              "TrueHD audio",
	StreamTypeSCTE35:                   "SCTE-35 splice_info_section",
	StreamTypeEAC3Audio:                "E-AC-3 audio (ATSC)",
}

func (t StreamType) String() (s string) {
	var ok bool
	if s, ok = streamTypeNames[t]; !ok {
		s = fmt.Sprintf("0x%02x", uint8(t))
	}
	return
}

func (t StreamType) MarshalJSON() (b []byte, err error) {
	return json.Marshal(t.String())
}

func (t *StreamType) UnmarshalJSON(b []byte) (err error) {
	*t, err = util.UnmarshalEnum(b, streamTypeNames)
	return
}

func (t StreamType) ToPESStreamID() pes.StreamID {
	switch t {
	case StreamTypeMPEG1Video, StreamTypeMPEG2Video, StreamTypeMPEG4Video, StreamTypeH264Video,
		StreamTypeAuxiliaryVideo, StreamTypeSVCVideo, StreamTypeMVCVideo,
		StreamTypeMPEG2AdditionalViewVideo, StreamTypeH264AdditionalViewVideo,
		StreamTypeH265Video, StreamTypeHEVCTemporalVideo, StreamTypeMVCDVideo,
		StreamTypeHEVCEnhancementG, StreamTypeHEVCTemporalEnhancementG,
		StreamTypeHEVCEnhancementH, StreamTypeHEVCTemporalEnhancementH,
		StreamTypeVVCVideo, StreamTypeVVCTemporalVideo,
		StreamTypeEVCVideo, StreamTypeLCEVCVideo,
		StreamTypeCAVSVideo, StreamTypeVC1Video:
		return 0xe0
	case StreamTypeDIRACVideo:
		return 0xfd
	case StreamTypeMPEG1Audio, StreamTypeMPEG2Audio, StreamTypeAACAudio, StreamTypeAACLATMAudio, StreamTypeMPEG4Audio:
		return 0xc0
	case StreamTypeAC3Audio, StreamTypeEAC3Audio:
		return pes.StreamIDPrivateStream1
	case StreamTypePrivateSection, StreamTypePrivateData, StreamTypeMetadata:
		return 0xfc
	default:
		return 0xbd
	}
}
