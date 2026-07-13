package psi

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/k-danil/go-astits/v2/descriptor"
	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
	"github.com/k-danil/go-astits/v2/pes"
)

// StreamType is a PMT elementary-stream type (a video, audio or data codec).
type StreamType uint8

// Stream types
const (
	StreamTypeMPEG1Video                 StreamType = 0x01
	StreamTypeMPEG2Video                 StreamType = 0x02
	StreamTypeMPEG1Audio                 StreamType = 0x03 // ISO/IEC 11172-3
	StreamTypeMPEG2HalvedSampleRateAudio StreamType = 0x04 // ISO/IEC 13818-3
	StreamTypeMPEG2Audio                 StreamType = 0x04
	StreamTypePrivateSection             StreamType = 0x05
	StreamTypePrivateData                StreamType = 0x06
	StreamTypeMPEG2PacketizedData        StreamType = 0x06 // Rec. ITU-T H.222 | ISO/IEC 13818-1 i.e., DVB subtitles/VBI and AC-3
	StreamTypeADTS                       StreamType = 0x0F // ISO/IEC 13818-7 Audio with ADTS transport syntax
	StreamTypeAACAudio                   StreamType = 0x0f
	StreamTypeMPEG4Video                 StreamType = 0x10
	StreamTypeAACLATMAudio               StreamType = 0x11
	StreamTypeMetadata                   StreamType = 0x15
	StreamTypeH264Video                  StreamType = 0x1B // Rec. ITU-T H.264 | ISO/IEC 14496-10
	StreamTypeH265Video                  StreamType = 0x24 // Rec. ITU-T H.265 | ISO/IEC 23008-2
	StreamTypeHEVCVideo                  StreamType = 0x24
	StreamTypeCAVSVideo                  StreamType = 0x42
	StreamTypeVC1Video                   StreamType = 0xea
	StreamTypeDIRACVideo                 StreamType = 0xd1
	StreamTypeAC3Audio                   StreamType = 0x81
	StreamTypeDTSAudio                   StreamType = 0x82
	StreamTypeTRUEHDAudio                StreamType = 0x83
	StreamTypeSCTE35                     StreamType = 0x86
	StreamTypeEAC3Audio                  StreamType = 0x87
)

// PMT represents a PMT data
// https://en.wikipedia.org/wiki/Program-specific_information
type PMT struct {
	ElementaryStreams  []ElementaryStream      `json:"_elementary_streams"`
	ProgramDescriptors []descriptor.Descriptor `json:"_program_descriptors"` // Program descriptors
	ProgramNumber      uint16                  `json:"program_number"`
	PCRPID             uint16                  `json:"PCR_PID"` // The packet identifier that contains the program clock reference used to improve the random access accuracy of the stream's timing that is derived from the program timestamp. If this is unused. then it is set to 0x1FFF (all bits on).
}

// ElementaryStream represents a PMT elementary stream
type ElementaryStream struct {
	ElementaryStreamDescriptors []descriptor.Descriptor `json:"_elementary_stream_descriptors"` // Elementary stream descriptors
	ElementaryPID               uint16                  `json:"elementary_PID"`                 // The packet identifier that contains the stream type data.
	StreamType                  StreamType              `json:"stream_type"`                    // This defines the structure of the data contained within the elementary packet identifier.
}

// parsePMTSection parses a PMT section
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
	// A PMT must fit a single section by spec (section_number shall be 0x00);
	// oversize data is rejected by the section length guard.

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
		StreamTypeH265Video,
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
		StreamTypeAC3Audio,
		StreamTypeDTSAudio,
		StreamTypeTRUEHDAudio,
		StreamTypeEAC3Audio:
		return true
	}
	return false
}

var streamTypeNames = map[StreamType]string{
	StreamTypeMPEG1Video:     "MPEG-1 video",
	StreamTypeMPEG2Video:     "MPEG-2 video",
	StreamTypeMPEG1Audio:     "MPEG-1 audio",
	StreamTypeMPEG2Audio:     "MPEG-2 audio",
	StreamTypePrivateSection: "private_sections",
	StreamTypePrivateData:    "PES private data",
	StreamTypeAACAudio:       "MPEG-2 AAC (ADTS)",
	StreamTypeMPEG4Video:     "MPEG-4 video",
	StreamTypeAACLATMAudio:   "MPEG-4 AAC (LATM)",
	StreamTypeMetadata:       "metadata in PES",
	StreamTypeH264Video:      "AVC video",
	StreamTypeH265Video:      "HEVC video",
	StreamTypeCAVSVideo:      "CAVS video",
	StreamTypeVC1Video:       "VC-1 video",
	StreamTypeDIRACVideo:     "Dirac video",
	StreamTypeAC3Audio:       "AC-3 audio (ATSC)",
	StreamTypeDTSAudio:       "DTS audio",
	StreamTypeTRUEHDAudio:    "TrueHD audio",
	StreamTypeSCTE35:         "SCTE-35 splice_info_section",
	StreamTypeEAC3Audio:      "E-AC-3 audio (ATSC)",
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
		StreamTypeH265Video, StreamTypeCAVSVideo, StreamTypeVC1Video:
		return 0xe0
	case StreamTypeDIRACVideo:
		return 0xfd
	case StreamTypeMPEG2Audio, StreamTypeAACAudio, StreamTypeAACLATMAudio:
		return 0xc0
	case StreamTypeAC3Audio, StreamTypeEAC3Audio: // m2ts_mode???
		return 0xfd
	case StreamTypePrivateSection, StreamTypePrivateData, StreamTypeMetadata:
		return 0xfc
	default:
		return 0xbd
	}
}
