package pes

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/k-danil/go-astits/v2/internal/util"
	"github.com/k-danil/go-astits/v2/ts"
)

// P-STD buffer scales
type PSTDBufferScale uint8

const (
	PSTDBufferScale128Bytes  PSTDBufferScale = 0
	PSTDBufferScale1024Bytes PSTDBufferScale = 1
)

var pstdBufferScaleNames = map[PSTDBufferScale]string{
	PSTDBufferScale128Bytes:  "128_bytes",
	PSTDBufferScale1024Bytes: "1024_bytes",
}

func (t PSTDBufferScale) String() (s string) {
	var ok bool
	if s, ok = pstdBufferScaleNames[t]; !ok {
		s = fmt.Sprintf("0x%02x", uint8(t))
	}
	return
}

func (t PSTDBufferScale) MarshalJSON() (b []byte, err error) {
	return json.Marshal(t.String())
}

func (t *PSTDBufferScale) UnmarshalJSON(b []byte) (err error) {
	*t, err = util.UnmarshalEnum(b, pstdBufferScaleNames)
	return
}

// PTS DTS indicator
type PTSDTSIndicator uint8

const (
	PTSDTSIndicatorBothPresent PTSDTSIndicator = 3
	PTSDTSIndicatorIsForbidden PTSDTSIndicator = 1
	PTSDTSIndicatorNoPTSOrDTS  PTSDTSIndicator = 0
	PTSDTSIndicatorOnlyPTS     PTSDTSIndicator = 2
)

var ptsDTSIndicatorNames = map[PTSDTSIndicator]string{
	PTSDTSIndicatorNoPTSOrDTS:  "no_PTS_or_DTS",
	PTSDTSIndicatorIsForbidden: "forbidden",
	PTSDTSIndicatorOnlyPTS:     "PTS_only",
	PTSDTSIndicatorBothPresent: "PTS_and_DTS",
}

func (t PTSDTSIndicator) String() (s string) {
	var ok bool
	if s, ok = ptsDTSIndicatorNames[t]; !ok {
		s = fmt.Sprintf("0x%02x", uint8(t))
	}
	return
}

func (t PTSDTSIndicator) MarshalJSON() (b []byte, err error) {
	return json.Marshal(t.String())
}

func (t *PTSDTSIndicator) UnmarshalJSON(b []byte) (err error) {
	*t, err = util.UnmarshalEnum(b, ptsDTSIndicatorNames)
	return
}

// Stream IDs
type StreamID uint8

const (
	StreamIDProgramStreamMap       StreamID = 0xbc
	StreamIDPrivateStream1         StreamID = 0xbd
	StreamIDPaddingStream          StreamID = 0xbe
	StreamIDPrivateStream2         StreamID = 0xbf
	StreamIDECM                    StreamID = 0xf0
	StreamIDEMM                    StreamID = 0xf1
	StreamIDDSMCC                  StreamID = 0xf2
	StreamIDH2221TypeE             StreamID = 0xf8
	StreamIDProgramStreamDirectory StreamID = 0xff
)

const (
	streamIDAudioBase       StreamID = 0xc0
	streamIDAudioNumberMask StreamID = 0x1f
	streamIDVideoBase       StreamID = 0xe0
	streamIDVideoNumberMask StreamID = 0x0f
)

const (
	streamIDAudioPrefix = "audio_stream_"
	streamIDVideoPrefix = "video_stream_"
)

var streamIDNames = map[StreamID]string{
	StreamIDProgramStreamMap:       "program_stream_map",
	StreamIDPrivateStream1:         "private_stream_1",
	StreamIDPaddingStream:          "padding_stream",
	StreamIDPrivateStream2:         "private_stream_2",
	StreamIDECM:                    "ECM_stream",
	StreamIDEMM:                    "EMM_stream",
	StreamIDDSMCC:                  "DSMCC_stream",
	StreamIDH2221TypeE:             "H.222.1_type_E",
	StreamIDProgramStreamDirectory: "program_stream_directory",
}

func (t StreamID) String() (s string) {
	var ok bool
	if s, ok = streamIDNames[t]; ok {
		return
	}
	switch {
	case t&^streamIDAudioNumberMask == streamIDAudioBase:
		s = fmt.Sprintf("%s%d", streamIDAudioPrefix, uint8(t&streamIDAudioNumberMask))
	case t&^streamIDVideoNumberMask == streamIDVideoBase:
		s = fmt.Sprintf("%s%d", streamIDVideoPrefix, uint8(t&streamIDVideoNumberMask))
	default:
		s = fmt.Sprintf("0x%02x", uint8(t))
	}
	return
}

func (t StreamID) MarshalJSON() (b []byte, err error) {
	return json.Marshal(t.String())
}

func (t *StreamID) UnmarshalJSON(b []byte) (err error) {
	if *t, err = util.UnmarshalEnum(b, streamIDNames); err == nil {
		return
	}
	var s string
	if json.Unmarshal(b, &s) != nil {
		return
	}
	if num, ok := strings.CutPrefix(s, streamIDAudioPrefix); ok {
		return t.fromStreamNumber(num, streamIDAudioBase, streamIDAudioNumberMask)
	}
	if num, ok := strings.CutPrefix(s, streamIDVideoPrefix); ok {
		return t.fromStreamNumber(num, streamIDVideoBase, streamIDVideoNumberMask)
	}
	return
}

func (t *StreamID) fromStreamNumber(num string, base, mask StreamID) (err error) {
	var n uint64
	if n, err = strconv.ParseUint(num, 10, 8); err != nil || StreamID(n)&^mask != 0 {
		err = fmt.Errorf("astits: invalid stream number %q", num)
		return
	}
	*t = base | StreamID(n)
	return
}

// Trick mode controls
type TrickModeControl uint8

const (
	TrickModeControlFastForward TrickModeControl = 0
	TrickModeControlFastReverse TrickModeControl = 3
	TrickModeControlFreezeFrame TrickModeControl = 2
	TrickModeControlSlowMotion  TrickModeControl = 1
	TrickModeControlSlowReverse TrickModeControl = 4
)

var trickModeControlNames = map[TrickModeControl]string{
	TrickModeControlFastForward: "fast_forward",
	TrickModeControlSlowMotion:  "slow_motion",
	TrickModeControlFreezeFrame: "freeze_frame",
	TrickModeControlFastReverse: "fast_reverse",
	TrickModeControlSlowReverse: "slow_reverse",
}

func (t TrickModeControl) String() (s string) {
	var ok bool
	if s, ok = trickModeControlNames[t]; !ok {
		s = fmt.Sprintf("0x%02x", uint8(t))
	}
	return
}

func (t TrickModeControl) MarshalJSON() (b []byte, err error) {
	return json.Marshal(t.String())
}

func (t *TrickModeControl) UnmarshalJSON(b []byte) (err error) {
	*t, err = util.UnmarshalEnum(b, trickModeControlNames)
	return
}

// PES scrambling controls
type ScramblingControl uint8

const (
	ScramblingControlNotScrambled ScramblingControl = 0
	ScramblingControlUserDefined1 ScramblingControl = 1
	ScramblingControlUserDefined2 ScramblingControl = 2
	ScramblingControlUserDefined3 ScramblingControl = 3
)

var scramblingControlNames = map[ScramblingControl]string{
	ScramblingControlNotScrambled: "not_scrambled",
	ScramblingControlUserDefined1: "user_defined_1",
	ScramblingControlUserDefined2: "user_defined_2",
	ScramblingControlUserDefined3: "user_defined_3",
}

func (t ScramblingControl) String() (s string) {
	var ok bool
	if s, ok = scramblingControlNames[t]; !ok {
		s = fmt.Sprintf("0x%02x", uint8(t))
	}
	return
}

func (t ScramblingControl) MarshalJSON() (b []byte, err error) {
	return json.Marshal(t.String())
}

func (t *ScramblingControl) UnmarshalJSON(b []byte) (err error) {
	*t, err = util.UnmarshalEnum(b, scramblingControlNames)
	return
}

// Field IDs
type FieldID uint8

const (
	FieldIDTopFieldOnly    FieldID = 0
	FieldIDBottomFieldOnly FieldID = 1
	FieldIDCompleteFrame   FieldID = 2
	FieldIDReserved        FieldID = 3
)

var fieldIDNames = map[FieldID]string{
	FieldIDTopFieldOnly:    "display_from_top_field_only",
	FieldIDBottomFieldOnly: "display_from_bottom_field_only",
	FieldIDCompleteFrame:   "display_complete_frame",
	FieldIDReserved:        "reserved",
}

func (t FieldID) String() (s string) {
	var ok bool
	if s, ok = fieldIDNames[t]; !ok {
		s = fmt.Sprintf("0x%02x", uint8(t))
	}
	return
}

func (t FieldID) MarshalJSON() (b []byte, err error) {
	return json.Marshal(t.String())
}

func (t *FieldID) UnmarshalJSON(b []byte) (err error) {
	*t, err = util.UnmarshalEnum(b, fieldIDNames)
	return
}

// Frequency truncations
type FrequencyTruncation uint8

const (
	FrequencyTruncationDCCoefficientsOnly     FrequencyTruncation = 0
	FrequencyTruncationFirstThreeCoefficients FrequencyTruncation = 1
	FrequencyTruncationFirstSixCoefficients   FrequencyTruncation = 2
	FrequencyTruncationAllCoefficients        FrequencyTruncation = 3
)

var frequencyTruncationNames = map[FrequencyTruncation]string{
	FrequencyTruncationDCCoefficientsOnly:     "only_DC_coefficients_non_zero",
	FrequencyTruncationFirstThreeCoefficients: "only_first_three_coefficients_non_zero",
	FrequencyTruncationFirstSixCoefficients:   "only_first_six_coefficients_non_zero",
	FrequencyTruncationAllCoefficients:        "all_coefficients_may_be_non_zero",
}

func (t FrequencyTruncation) String() (s string) {
	var ok bool
	if s, ok = frequencyTruncationNames[t]; !ok {
		s = fmt.Sprintf("0x%02x", uint8(t))
	}
	return
}

func (t FrequencyTruncation) MarshalJSON() (b []byte, err error) {
	return json.Marshal(t.String())
}

func (t *FrequencyTruncation) UnmarshalJSON(b []byte) (err error) {
	*t, err = util.UnmarshalEnum(b, frequencyTruncationNames)
	return
}

const (
	HeaderSize         = 6
	dsmTrickModeLength = 1
)

// Data represents a PES data
// https://en.wikipedia.org/wiki/Packetized_elementary_stream
// http://dvd.sourceforge.net/dvdinfo/pes-hdr.html
// http://happy.emu.id.au/lab/tut/dttb/dtbtut4b.htm
type Data struct {
	Data   []byte `json:"PES_packet_data_byte"`
	Header Header `json:"_header"`
}

// Header represents a packet PES header
type Header struct {
	OptionalHeader *OptionalHeader `json:"_optional_header"`
	optionalHeader OptionalHeader  // storage for OptionalHeader — no allocation per PES
	PacketLength   uint16          `json:"PES_packet_length"` // Specifies the number of bytes remaining in the packet after this field. Can be zero. If the PES packet length is set to zero, the PES packet can be of any length. A value of zero for the PES packet length can be used only when the PES packet payload is a video elementary stream.
	StreamID       StreamID        `json:"stream_id"`         // Examples: Audio streams (0xC0-0xDF), Video streams (0xE0-0xEF)
}

// OptionalHeader represents a PES optional header
type OptionalHeader struct {
	DSMTrickMode           *DSMTrickMode            `json:"DSM_trick_mode"`
	Extension              *OptionalHeaderExtension `json:"_extension"`
	DTS                    ts.ClockReference        `json:"DTS"`
	PTS                    ts.ClockReference        `json:"PTS"`
	ESCR                   ts.ClockReference        `json:"ESCR"`
	ESRate                 uint32                   `json:"ES_rate"`
	CRC                    uint16                   `json:"previous_PES_packet_CRC"`
	AdditionalCopyInfo     uint8                    `json:"additional_copy_info"`
	DataAlignmentIndicator bool                     `json:"data_alignment_indicator"` // True indicates that the PES packet header is immediately followed by the video start code or audio syncword
	HasAdditionalCopyInfo  bool                     `json:"additional_copy_info_flag"`
	HasCRC                 bool                     `json:"PES_CRC_flag"`
	HasDSMTrickMode        bool                     `json:"DSM_trick_mode_flag"`
	HasESCR                bool                     `json:"ESCR_flag"`
	HasESRate              bool                     `json:"ES_rate_flag"`
	HasExtension           bool                     `json:"PES_extension_flag"`
	HasOptionalFields      bool                     `json:"_has_optional_fields"`
	HeaderLength           uint8                    `json:"PES_header_data_length"`
	IsCopyrighted          bool                     `json:"copyright"`
	IsOriginal             bool                     `json:"original_or_copy"`
	MarkerBits             uint8                    `json:"_marker_bits"`
	Priority               bool                     `json:"PES_priority"`
	PTSDTSIndicator        PTSDTSIndicator          `json:"PTS_DTS_flags"`
	ScramblingControl      ScramblingControl        `json:"PES_scrambling_control"`
}

type OptionalHeaderExtension struct {
	PrivateData                     []byte            `json:"PES_private_data"`
	Extension2Reserved              []byte            `json:"_extension_2_reserved"`
	PackHeader                      []byte            `json:"pack_header"`
	TREF                            ts.ClockReference `json:"TREF"`
	HasPrivateData                  bool              `json:"PES_private_data_flag"`
	HasPackHeaderField              bool              `json:"pack_header_field_flag"`
	HasProgramPacketSequenceCounter bool              `json:"program_packet_sequence_counter_flag"`
	HasPSTDBuffer                   bool              `json:"P-STD_buffer_flag"`
	HasExtension2                   bool              `json:"PES_extension_flag_2"`
	HasStreamIDExtension            bool              `json:"stream_id_extension_flag"`
	HasTREF                         bool              `json:"tref_extension_flag"`
	PackField                       uint8             `json:"pack_field_length"`
	PacketSequenceCounter           uint8             `json:"program_packet_sequence_counter"`
	MPEG1OrMPEG2ID                  uint8             `json:"MPEG1_MPEG2_identifier"`
	OriginalStuffingLength          uint8             `json:"original_stuff_length"`
	PSTDBufferScale                 PSTDBufferScale   `json:"P-STD_buffer_scale"`
	PSTDBufferSize                  uint16            `json:"P-STD_buffer_size"`
	StreamIDExtension               uint8             `json:"stream_id_extension"`
}

// TREF shares the PTS/DTS wire layout; its top nibble is reserved, not a prefix.
// H.222.0 §2.4.3.7
const trefReservedPrefix = 0b1111

// DSMTrickMode represents a DSM trick mode
// https://books.google.fr/books?id=vwUrAwAAQBAJ&pg=PT501&lpg=PT501&dq=dsm+trick+mode+control&source=bl&ots=fI-9IHXMRL&sig=PWnhxrsoMWNQcl1rMCPmJGNO9Ds&hl=fr&sa=X&ved=0ahUKEwjogafD8bjXAhVQ3KQKHeHKD5oQ6AEINDAB#v=onepage&q=dsm%20trick%20mode%20control&f=false
type DSMTrickMode struct {
	FieldID             FieldID             `json:"field_id"`
	FrequencyTruncation FrequencyTruncation `json:"frequency_truncation"`
	IntraSliceRefresh   uint8               `json:"intra_slice_refresh"`
	RepeatControl       uint8               `json:"rep_cntrl"`
	TrickModeControl    TrickModeControl    `json:"trick_mode_control"`
}

func (h *Header) IsVideoStream() bool {
	return h.StreamID == 0xe0 ||
		h.StreamID == 0xfd
}

// Parse parses a PES data
func (d *Data) Parse(bs []byte) (err error) {
	const pesPayloadPrefixSize = 3

	var dataStart, dataEnd int
	if dataStart, dataEnd, err = d.Header.parseBytes(bs, pesPayloadPrefixSize); err != nil {
		err = fmt.Errorf("astits: parsing PES header failed: %w", err)
		return
	}

	if dataEnd < dataStart {
		err = fmt.Errorf("astits: data end %d is before data start %d: %w", dataEnd, dataStart, ts.ErrInvalidData)
		return
	}
	if dataStart > len(bs) || dataEnd > len(bs) {
		return ts.ErrShortPacket
	}

	d.Data = bs[dataStart:dataEnd]
	return
}

// hasPESOptionalHeader reports whether the PES packet carries the optional
// header. Per H.222.0 Table 2-21 it is absent for these stream_ids only.
func hasPESOptionalHeader(streamID StreamID) bool {
	switch streamID {
	case StreamIDProgramStreamMap, StreamIDPaddingStream, StreamIDPrivateStream2,
		StreamIDECM, StreamIDEMM, StreamIDDSMCC, StreamIDH2221TypeE, StreamIDProgramStreamDirectory:
		return false
	}
	return true
}

// parseBytes parses a PES header starting at bs[o]
func (h *Header) parseBytes(bs []byte, o int) (dataStart, dataEnd int, err error) {
	if o+HeaderSize-3 > len(bs) {
		return 0, 0, ts.ErrShortPacket
	}

	h.StreamID = StreamID(bs[o])
	h.PacketLength = binary.BigEndian.Uint16(bs[o+1 : o+3])
	o += 3

	if h.PacketLength > 0 {
		dataEnd = o + int(h.PacketLength)
	} else {
		dataEnd = len(bs)
	}

	if hasPESOptionalHeader(h.StreamID) {
		h.optionalHeader = OptionalHeader{}
		h.OptionalHeader = &h.optionalHeader
		if dataStart, err = h.OptionalHeader.parseBytes(bs, o); err != nil {
			err = fmt.Errorf("astits: parsing PES optional header failed: %w", err)
			return
		}
	} else {
		dataStart = o
	}
	return
}

// parseBytes parses a PES optional header starting at bs[o]
func (h *OptionalHeader) parseBytes(bs []byte, o int) (dataStart int, err error) {
	if o+3 > len(bs) {
		return 0, ts.ErrShortPacket
	}

	b := bs[o]
	h.MarkerBits = b >> 6
	h.ScramblingControl = ScramblingControl(b >> 4 & 0x3)
	h.Priority = b&0x8 > 0
	h.DataAlignmentIndicator = b&0x4 > 0
	h.IsCopyrighted = b&0x2 > 0
	h.IsOriginal = b&0x1 > 0
	b = bs[o+1]
	h.PTSDTSIndicator = PTSDTSIndicator(b >> 6 & 0x3)

	h.HasESCR = b&0x20 > 0
	h.HasESRate = b&0x10 > 0
	h.HasDSMTrickMode = b&0x8 > 0
	h.HasAdditionalCopyInfo = b&0x4 > 0
	h.HasCRC = b&0x2 > 0
	h.HasExtension = b&0x1 > 0

	h.HeaderLength = bs[o+2]
	o += 3

	dataStart = o + int(h.HeaderLength)

	var n int
	switch h.PTSDTSIndicator {
	case PTSDTSIndicatorOnlyPTS:
		if n, err = h.PTS.ParsePTSDTS(bs[o:]); err != nil {
			err = fmt.Errorf("astits: parsing PTS failed: %w", err)
			return
		}
		o += n
	case PTSDTSIndicatorBothPresent:
		if n, err = h.PTS.ParsePTSDTS(bs[o:]); err != nil {
			err = fmt.Errorf("astits: parsing PTS failed: %w", err)
			return
		}
		o += n
		if n, err = h.DTS.ParsePTSDTS(bs[o:]); err != nil {
			err = fmt.Errorf("astits: parsing DTS failed: %w", err)
			return
		}
		o += n
	}

	if h.HasESCR {
		if n, err = h.ESCR.ParseESCR(bs[o:]); err != nil {
			err = fmt.Errorf("astits: parsing ESCR failed: %w", err)
			return
		}
		o += n
	}

	if h.HasESRate {
		if o+3 > len(bs) {
			return 0, ts.ErrShortPacket
		}
		h.ESRate = uint32(bs[o])&0x7f<<15 | uint32(bs[o+1])<<7 | uint32(bs[o+2])>>1
		o += 3
	}

	if h.HasDSMTrickMode {
		if o >= len(bs) {
			return 0, ts.ErrShortPacket
		}
		h.DSMTrickMode = parseDSMTrickMode(bs[o])
		o++
	}

	if h.HasAdditionalCopyInfo {
		if o >= len(bs) {
			return 0, ts.ErrShortPacket
		}
		h.AdditionalCopyInfo = bs[o] & 0x7f
		o++
	}

	if h.HasCRC {
		if o+2 > len(bs) {
			return 0, ts.ErrShortPacket
		}
		h.CRC = binary.BigEndian.Uint16(bs[o:])
		o += 2
	}

	if h.HasExtension {
		h.Extension = &OptionalHeaderExtension{}
		err = h.Extension.parseBytes(bs, o)
		return
	}

	return
}

func (h *OptionalHeaderExtension) parseBytes(bs []byte, o int) (err error) {
	if o >= len(bs) {
		return ts.ErrShortPacket
	}
	b := bs[o]
	o++

	h.HasPrivateData = b&0x80 > 0
	h.HasPackHeaderField = b&0x40 > 0
	h.HasProgramPacketSequenceCounter = b&0x20 > 0
	h.HasPSTDBuffer = b&0x10 > 0
	h.HasExtension2 = b&0x1 > 0

	if h.HasPrivateData {
		if o+16 > len(bs) {
			return ts.ErrShortPacket
		}
		h.PrivateData = bs[o : o+16]
		o += 16
	}

	if h.HasPackHeaderField {
		if o >= len(bs) {
			return ts.ErrShortPacket
		}
		h.PackField = bs[o]
		o++
		if o+int(h.PackField) > len(bs) {
			return ts.ErrShortPacket
		}
		h.PackHeader = bs[o : o+int(h.PackField)]
		o += int(h.PackField)
	}

	if h.HasProgramPacketSequenceCounter {
		if o+2 > len(bs) {
			return ts.ErrShortPacket
		}
		h.PacketSequenceCounter = bs[o] & 0x7f
		h.MPEG1OrMPEG2ID = bs[o+1] >> 6 & 0x1
		h.OriginalStuffingLength = bs[o+1] & 0x3f
		o += 2
	}

	if h.HasPSTDBuffer {
		if o+2 > len(bs) {
			return ts.ErrShortPacket
		}
		h.PSTDBufferScale = PSTDBufferScale(bs[o] >> 5 & 0x1)
		h.PSTDBufferSize = binary.BigEndian.Uint16(bs[o:]) & 0x1fff
		o += 2
	}

	if h.HasExtension2 {
		if o >= len(bs) {
			return ts.ErrShortPacket
		}
		fieldLen := int(bs[o] & 0x7f)
		o++
		if o+fieldLen > len(bs) {
			return ts.ErrShortPacket
		}
		fieldEnd := o + fieldLen

		if fieldLen > 0 {
			b = bs[o]
			o++
			if h.HasStreamIDExtension = b&0x80 == 0; h.HasStreamIDExtension {
				h.StreamIDExtension = b & 0x7f
			} else if h.HasTREF = b&0x01 == 0; h.HasTREF {
				var n int
				if n, err = h.TREF.ParsePTSDTS(bs[o:fieldEnd]); err != nil {
					err = fmt.Errorf("astits: parsing TREF failed: %w", err)
					return
				}
				o += n
			}
			h.Extension2Reserved = bs[o:fieldEnd]
		}
	}
	return
}

// parseDSMTrickMode parses a DSM trick mode
func parseDSMTrickMode(i byte) (m *DSMTrickMode) {
	m = &DSMTrickMode{}
	m.TrickModeControl = TrickModeControl(i >> 5)
	switch m.TrickModeControl {
	case TrickModeControlFastForward, TrickModeControlFastReverse:
		m.FieldID = FieldID(i >> 3 & 0x3)
		m.IntraSliceRefresh = i >> 2 & 0x1
		m.FrequencyTruncation = FrequencyTruncation(i & 0x3)
	case TrickModeControlFreezeFrame:
		m.FieldID = FieldID(i >> 3 & 0x3)
	case TrickModeControlSlowMotion, TrickModeControlSlowReverse:
		m.RepeatControl = i & 0x1f
	}
	return
}

// CalcDataLength returns how many total and payload bytes Put would write for
// the same arguments. The counts must stay identical to Put so a muxer can
// finalize the AF stuffing before serializing the PES. Only the first packet
// of a unit carries the PES header.
func (h *Header) CalcDataLength(payloadLeft []byte, isPayloadStart bool, bytesAvailable int) (totalBytes, payloadBytes int) {
	headerBytes := 0
	if isPayloadStart {
		headerBytes = HeaderSize
		if hasPESOptionalHeader(h.StreamID) {
			headerBytes += h.OptionalHeader.CalcLength()
		}
	}

	payloadBytes = bytesAvailable - headerBytes
	if len(payloadLeft) < payloadBytes {
		payloadBytes = len(payloadLeft)
	}

	totalBytes = headerBytes + payloadBytes
	return
}

// first packet will contain PES header with optional PES header and payload, if possible
// all consequential packets will contain just payload
// for the last packet caller must add AF with stuffing, see calcPESDataLength
func (h *Header) Put(bs []byte, payloadLeft []byte, isPayloadStart bool) (totalBytesWritten, payloadBytesWritten int, err error) {
	if isPayloadStart {
		var n int
		if n, err = h.putBytes(bs, len(payloadLeft)); err != nil {
			return
		}
		totalBytesWritten += n
	}

	payloadBytesWritten = len(bs) - totalBytesWritten
	if payloadBytesWritten > len(payloadLeft) {
		payloadBytesWritten = len(payloadLeft)
	}

	copy(bs[totalBytesWritten:], payloadLeft[:payloadBytesWritten])
	totalBytesWritten += payloadBytesWritten
	return
}

// PutHeader serializes just the PES header (start code, stream_id, PES packet
// length and, when present, the optional header) into bs, sizing PES_packet_length
// for payloadLen total payload bytes. It returns the number of header bytes
// written; a muxer that spans the header across TS packets serializes it once
// here and then chunks header+payload together.
func (h *Header) PutHeader(bs []byte, payloadLen int) (n int, err error) {
	return h.putBytes(bs, payloadLen)
}

func (h *Header) putBytes(bs []byte, payloadSize int) (n int, err error) {
	if len(bs) < HeaderSize {
		return 0, ts.ErrShortPacket
	}
	binary.BigEndian.PutUint32(bs, uint32(h.StreamID)|0x1<<8)
	pesPacketLength := 0

	if !h.IsVideoStream() {
		pesPacketLength = payloadSize
		if hasPESOptionalHeader(h.StreamID) {
			pesPacketLength += h.OptionalHeader.CalcLength()
		}
		pesPacketLength *= int((uint64(pesPacketLength) - 0x10000) >> 63)
	}

	binary.BigEndian.PutUint16(bs[4:], uint16(pesPacketLength))
	n = HeaderSize

	if hasPESOptionalHeader(h.StreamID) {
		n += h.OptionalHeader.putBytes(bs[n:])
	}

	return
}

func (h *OptionalHeader) CalcLength() int {
	if h == nil {
		return 0
	}
	return 3 + int(h.calcDataLength())
}

func (h *OptionalHeader) calcDataLength() (length uint8) {
	switch h.PTSDTSIndicator {
	case PTSDTSIndicatorOnlyPTS:
		length += ts.PTSDTSSize
	case PTSDTSIndicatorBothPresent:
		length += 2 * ts.PTSDTSSize
	}

	length += ts.ESCRSize * util.B2U(h.HasESCR)
	length += 3 * util.B2U(h.HasESRate)
	length += dsmTrickModeLength * util.B2U(h.HasDSMTrickMode)
	length += util.B2U(h.HasAdditionalCopyInfo)
	length += 2 * util.B2U(h.HasCRC)

	if h.HasExtension {
		length += h.Extension.calcDataLength()
	}
	return
}

func (h *OptionalHeaderExtension) calcDataLength() (length uint8) {
	length++
	length += 16 * util.B2U(h.HasPrivateData)
	if h.HasPackHeaderField {
		length += 1 + uint8(len(h.PackHeader))
	}
	length += 2 * util.B2U(h.HasProgramPacketSequenceCounter)
	length += 2 * util.B2U(h.HasPSTDBuffer)
	if h.HasExtension2 {
		length += 2 + uint8(len(h.Extension2Reserved))
		if !h.HasStreamIDExtension && h.HasTREF {
			length += ts.PTSDTSSize
		}
	}
	return
}

func (h *OptionalHeader) putBytes(bs []byte) (n int) {
	if h == nil {
		return 0
	}

	b := uint8(0b10) << 6
	b |= uint8(h.ScramblingControl) << 4
	b |= util.B2U(h.Priority) << 3
	b |= util.B2U(h.DataAlignmentIndicator) << 2
	b |= util.B2U(h.IsCopyrighted) << 1
	b |= util.B2U(h.IsOriginal)
	bs[0] = b
	b = uint8(h.PTSDTSIndicator) << 6
	b |= util.B2U(h.HasESCR) << 5
	b |= util.B2U(h.HasESRate) << 4
	b |= util.B2U(h.HasDSMTrickMode) << 3
	b |= util.B2U(h.HasAdditionalCopyInfo) << 2
	b |= util.B2U(h.HasCRC) << 1
	b |= util.B2U(h.HasExtension)
	bs[1] = b
	bs[2] = h.calcDataLength()
	n = 3

	if h.PTSDTSIndicator == PTSDTSIndicatorOnlyPTS {
		n += h.PTS.PutPTSDTS(bs[n:], 0b0010)
	}

	if h.PTSDTSIndicator == PTSDTSIndicatorBothPresent {
		n += h.PTS.PutPTSDTS(bs[n:], 0b0011)
		n += h.DTS.PutPTSDTS(bs[n:], 0b0001)
	}

	if h.HasESCR {
		n += h.ESCR.PutESCR(bs[n:])
	}

	if h.HasESRate {
		bs[n] = 0x80 | uint8(h.ESRate>>15)
		bs[n+1] = uint8(h.ESRate >> 7)
		bs[n+2] = uint8(h.ESRate<<1) | 0x1
		n += 3
	}

	if h.HasDSMTrickMode {
		n += h.DSMTrickMode.putBytes(bs[n:])
	}

	if h.HasAdditionalCopyInfo {
		bs[n] = 0x80 | h.AdditionalCopyInfo
		n++
	}

	if h.HasCRC {
		binary.BigEndian.PutUint16(bs[n:], h.CRC)
		n += 2
	}

	if h.HasExtension {
		n += h.Extension.putBytes(bs[n:])
	}

	return
}

func (h *OptionalHeaderExtension) putBytes(bs []byte) (n int) {
	// exp 10110001
	// act 10111111
	bs[0] = util.B2U(h.HasPrivateData) << 7
	bs[0] |= util.B2U(h.HasPackHeaderField) << 6
	bs[0] |= util.B2U(h.HasProgramPacketSequenceCounter) << 5
	bs[0] |= util.B2U(h.HasPSTDBuffer) << 4
	bs[0] |= 0xe
	bs[0] |= util.B2U(h.HasExtension2)
	n = 1

	if h.HasPrivateData {
		// like WriteBytesN: exactly 16 bytes, the remainder padded with zeros
		c := copy(bs[n:n+16], h.PrivateData)
		for i := n + c; i < n+16; i++ {
			bs[i] = 0
		}
		n += 16
	}

	if h.HasPackHeaderField {
		bs[n] = uint8(len(h.PackHeader))
		n++
		n += copy(bs[n:], h.PackHeader)
	}

	if h.HasProgramPacketSequenceCounter {
		bs[n] = 0x80 | h.PacketSequenceCounter
		bs[n+1] = 0x80 | h.MPEG1OrMPEG2ID<<6 | h.OriginalStuffingLength
		n += 2
	}

	if h.HasPSTDBuffer {
		bs[n] = 0x40 | uint8(h.PSTDBufferScale)<<5 | uint8(h.PSTDBufferSize>>8)
		bs[n+1] = uint8(h.PSTDBufferSize)
		n += 2
	}

	if h.HasExtension2 {
		fieldLen := 1 + len(h.Extension2Reserved)
		if !h.HasStreamIDExtension && h.HasTREF {
			fieldLen += ts.PTSDTSSize
		}
		bs[n] = 0x80 | uint8(fieldLen)
		n++
		if h.HasStreamIDExtension {
			bs[n] = h.StreamIDExtension & 0x7f
			n++
		} else {
			bs[n] = 0xfe | util.B2U(!h.HasTREF)
			n++
			if h.HasTREF {
				n += h.TREF.PutPTSDTS(bs[n:], trefReservedPrefix)
			}
		}
		n += copy(bs[n:], h.Extension2Reserved)
	}
	return
}

func (m *DSMTrickMode) putBytes(bs []byte) int {
	b := uint8(m.TrickModeControl) << 5

	switch m.TrickModeControl {
	case TrickModeControlFastForward, TrickModeControlFastReverse:
		b |= uint8(m.FieldID)<<3 | m.IntraSliceRefresh<<2 | uint8(m.FrequencyTruncation)
	case TrickModeControlFreezeFrame:
		b |= uint8(m.FieldID)<<3 | 7
	case TrickModeControlSlowMotion, TrickModeControlSlowReverse:
		b |= m.RepeatControl
	default:
		b |= 0x1f
	}

	bs[0] = b
	return dsmTrickModeLength
}
