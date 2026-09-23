package pes

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/k-danil/go-astits/v3/internal/errclass"
	"github.com/k-danil/go-astits/v3/internal/util"
	"github.com/k-danil/go-astits/v3/ts"
)

// The spec fixes these two bits at '10', so anything else is a misaligned or corrupt header.
const optionalHeaderMarker = 0b10

var (
	ErrInvalidMarkerBits     = errclass.New("astits: invalid PES optional header marker bits", ts.ErrInvalidData)
	ErrUnboundedNonVideo     = errclass.New("astits: unbounded PES packet on a non-video stream", ts.ErrInvalidData)
	ErrInvalidStartCode      = errclass.New("astits: invalid PES packet_start_code_prefix", ts.ErrInvalidData)
	ErrHeaderTooLong         = errclass.New("astits: PES optional header exceeds its length field", ts.ErrInvalidData)
	ErrMissingOptionalHeader = errclass.New("astits: stream_id requires a PES optional header", ts.ErrInvalidData)
	ErrMissingExtension      = errclass.New("astits: PES_extension_flag set without an extension header", ts.ErrInvalidData)
	ErrShortPayload          = errclass.New("astits: PES packet shorter than its PES_packet_length", ts.ErrInvalidData)
	ErrTrailingBytes         = errclass.New("astits: payload bytes past the PES_packet_length", ts.ErrInvalidData)
)

type ShortPayloadError struct{ Have, Want int }

func (e *ShortPayloadError) Error() string {
	return fmt.Sprintf("astits: PES packet is %d of its %d bytes", e.Have, e.Want)
}

func (e *ShortPayloadError) Unwrap() error { return ErrShortPayload }

type UnboundedLengthError struct{ StreamID StreamID }

func (e *UnboundedLengthError) Error() string {
	return fmt.Sprintf("astits: unbounded PES packet on non-video stream_id %s", e.StreamID)
}

func (e *UnboundedLengthError) Unwrap() error { return ErrUnboundedNonVideo }

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
	streamIDExtended        StreamID = 0xfd
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
	HeaderSize               = 6
	dsmTrickModeLength       = 1
	packetStartCodePrefix    = 0x000001
	optionalHeaderFixedLen   = 3
	maxOptionalHeaderData    = 0xff
	maxExtension2FieldLength = 0x7f
	maxPacketLength          = 0xffff
)

type Data struct {
	Data   []byte `json:"PES_packet_data_byte"`
	Header Header `json:"_header"`
}

type Header struct {
	OptionalHeader        *OptionalHeader `json:"_optional_header"`
	optionalHeaderStorage OptionalHeader
	PacketLength          uint16   `json:"PES_packet_length"`
	StreamID              StreamID `json:"stream_id"`
}

type OptionalHeader struct {
	DSMTrickMode           *DSMTrickMode            `json:"DSM_trick_mode"`
	Extension              *OptionalHeaderExtension `json:"_extension"`
	DTS                    ts.ClockReference        `json:"DTS"`
	PTS                    ts.ClockReference        `json:"PTS"`
	ESCR                   ts.ClockReference        `json:"ESCR"`
	ESRate                 uint32                   `json:"ES_rate"`
	CRC                    uint16                   `json:"previous_PES_packet_CRC"`
	AdditionalCopyInfo     uint8                    `json:"additional_copy_info"`
	DataAlignmentIndicator bool                     `json:"data_alignment_indicator"`
	HasAdditionalCopyInfo  bool                     `json:"additional_copy_info_flag"`
	HasCRC                 bool                     `json:"PES_CRC_flag"`
	HasDSMTrickMode        bool                     `json:"DSM_trick_mode_flag"`
	HasESCR                bool                     `json:"ESCR_flag"`
	HasESRate              bool                     `json:"ES_rate_flag"`
	HasExtension           bool                     `json:"PES_extension_flag"`
	HeaderLength           uint8                    `json:"PES_header_data_length"`
	IsCopyrighted          bool                     `json:"copyright"`
	IsOriginal             bool                     `json:"original_or_copy"`
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

// TREF's top nibble is reserved, not a prefix (H.222.0 §2.4.3.7).
const trefReservedPrefix = 0b1111

type DSMTrickMode struct {
	FieldID             FieldID             `json:"field_id"`
	FrequencyTruncation FrequencyTruncation `json:"frequency_truncation"`
	IntraSliceRefresh   uint8               `json:"intra_slice_refresh"`
	RepeatControl       uint8               `json:"rep_cntrl"`
	TrickModeControl    TrickModeControl    `json:"trick_mode_control"`
}

func (h *Header) IsVideoStream() bool {
	return h.StreamID&^streamIDVideoNumberMask == streamIDVideoBase
}

// §2.4.3.7 grants PES_packet_length 0 to video only; the extended stream_id carries VC-1/Dirac video too.
func AllowsUnboundedLength(id StreamID) bool {
	return id&^streamIDVideoNumberMask == streamIDVideoBase || id == streamIDExtended
}

func (d *Data) Parse(bs []byte) error {
	return d.parse(bs, false)
}

// Parse, but a declared length beyond bs clamps Data instead of failing.
func (d *Data) ParseTruncated(bs []byte) error {
	return d.parse(bs, true)
}

func (d *Data) parse(bs []byte, clamp bool) (err error) {
	const pesPayloadPrefixSize = 3

	if len(bs) < pesPayloadPrefixSize {
		return ts.ErrShortPacket
	}
	if uint32(bs[0])<<16|uint32(bs[1])<<8|uint32(bs[2]) != packetStartCodePrefix {
		return ErrInvalidStartCode
	}

	var dataStart, dataEnd int
	if dataStart, dataEnd, err = d.Header.parseBytes(bs, pesPayloadPrefixSize); err != nil {
		err = fmt.Errorf("astits: parsing PES header failed: %w", err)
		return
	}

	if dataStart > len(bs) {
		return ts.ErrShortPacket
	}
	if dataEnd < dataStart {
		err = fmt.Errorf("astits: data end %d is before data start %d: %w", dataEnd, dataStart, ts.ErrInvalidData)
		return
	}
	if dataEnd > len(bs) {
		if !clamp {
			return &ShortPayloadError{Have: len(bs), Want: dataEnd}
		}
		dataEnd = len(bs)
	}

	d.Data = bs[dataStart:dataEnd]
	return
}

// H.222.0 Table 2-21: the optional header is absent for these stream_ids only.
func hasPESOptionalHeader(streamID StreamID) bool {
	switch streamID {
	case StreamIDProgramStreamMap, StreamIDPaddingStream, StreamIDPrivateStream2,
		StreamIDECM, StreamIDEMM, StreamIDDSMCC, StreamIDH2221TypeE, StreamIDProgramStreamDirectory:
		return false
	}
	return true
}

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
		h.optionalHeaderStorage = OptionalHeader{}
		h.OptionalHeader = &h.optionalHeaderStorage
		if dataStart, err = h.OptionalHeader.parseBytes(bs, o); err != nil {
			err = fmt.Errorf("astits: parsing PES optional header failed: %w", err)
			return
		}
	} else {
		h.OptionalHeader = nil
		dataStart = o
	}
	return
}

func (h *OptionalHeader) parseBytes(bs []byte, o int) (dataStart int, err error) {
	if o+optionalHeaderFixedLen > len(bs) {
		return 0, ts.ErrShortPacket
	}

	b := bs[o]
	if b>>6 != optionalHeaderMarker {
		return 0, ErrInvalidMarkerBits
	}
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
	o += optionalHeaderFixedLen

	// PES_header_data_length bounds every field below: past it the bytes are payload (§2.4.3.7).
	if dataStart = o + int(h.HeaderLength); dataStart > len(bs) {
		return 0, ts.ErrShortPacket
	}
	bs = bs[:dataStart]

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

// Must return exactly what Put would write: callers size the adaptation-field stuffing from it.
func (h *Header) CalcDataLength(payloadLeft []byte, isPayloadStart bool, bytesAvailable int) (totalBytes, payloadBytes int) {
	headerBytes := 0
	if isPayloadStart {
		headerBytes = HeaderSize
		if hasPESOptionalHeader(h.StreamID) {
			headerBytes += h.OptionalHeader.CalcLength()
		}
	}

	payloadBytes = min(len(payloadLeft), bytesAvailable-headerBytes)

	totalBytes = headerBytes + payloadBytes
	return
}

// Only the first packet of a unit carries the PES header; the caller stuffs the last packet's adaptation field (see CalcDataLength).
func (h *Header) Put(bs []byte, payloadLeft []byte, isPayloadStart bool) (totalBytesWritten, payloadBytesWritten int, err error) {
	if isPayloadStart {
		var n int
		if n, err = h.putBytes(bs, len(payloadLeft)); err != nil {
			return
		}
		totalBytesWritten += n
	}

	payloadBytesWritten = min(len(bs)-totalBytesWritten, len(payloadLeft))

	copy(bs[totalBytesWritten:], payloadLeft[:payloadBytesWritten])
	totalBytesWritten += payloadBytesWritten
	return
}

// Serializes only the header, sizing PES_packet_length for payloadLen payload bytes.
func (h *Header) PutHeader(bs []byte, payloadLen int) (n int, err error) {
	return h.putBytes(bs, payloadLen)
}

func (h *Header) putBytes(bs []byte, payloadSize int) (n int, err error) {
	optionalLength := 0
	if hasPESOptionalHeader(h.StreamID) {
		if h.OptionalHeader == nil {
			return 0, fmt.Errorf("astits: stream_id %s: %w", h.StreamID, ErrMissingOptionalHeader)
		}
		if h.OptionalHeader.HasExtension && h.OptionalHeader.Extension == nil {
			return 0, ErrMissingExtension
		}
		optionalLength = h.OptionalHeader.CalcLength()
		if dataLength := optionalLength - optionalHeaderFixedLen; dataLength > maxOptionalHeaderData {
			return 0, fmt.Errorf("astits: PES header data length %d exceeds %d: %w", dataLength, maxOptionalHeaderData, ErrHeaderTooLong)
		}
		if ext := h.OptionalHeader.Extension; h.OptionalHeader.HasExtension && ext.HasExtension2 && ext.extension2FieldLength() > maxExtension2FieldLength {
			return 0, fmt.Errorf("astits: PES_extension_field_length %d exceeds %d: %w", ext.extension2FieldLength(), maxExtension2FieldLength, ErrHeaderTooLong)
		}
	}
	if len(bs) < HeaderSize+optionalLength {
		return 0, ts.ErrShortPacket
	}

	packetLength := 0
	if !AllowsUnboundedLength(h.StreamID) {
		if packetLength = payloadSize + optionalLength; packetLength > maxPacketLength {
			return 0, fmt.Errorf("astits: PES packet length %d exceeds %d on stream_id %s: %w", packetLength, maxPacketLength, h.StreamID, ErrUnboundedNonVideo)
		}
	}

	binary.BigEndian.PutUint32(bs, uint32(h.StreamID)|packetStartCodePrefix<<8)
	binary.BigEndian.PutUint16(bs[4:], uint16(packetLength))
	n = HeaderSize
	if optionalLength > 0 {
		n += h.OptionalHeader.putBytes(bs[n:])
	}
	return
}

func (h *OptionalHeader) CalcLength() int {
	if h == nil {
		return 0
	}
	return optionalHeaderFixedLen + h.calcDataLength()
}

func (h *OptionalHeader) calcDataLength() (length int) {
	switch h.PTSDTSIndicator {
	case PTSDTSIndicatorOnlyPTS:
		length += ts.PTSDTSSize
	case PTSDTSIndicatorBothPresent:
		length += 2 * ts.PTSDTSSize
	}

	length += ts.ESCRSize * int(util.B2U(h.HasESCR))
	length += 3 * int(util.B2U(h.HasESRate))
	length += dsmTrickModeLength * int(util.B2U(h.HasDSMTrickMode))
	length += int(util.B2U(h.HasAdditionalCopyInfo))
	length += 2 * int(util.B2U(h.HasCRC))

	if h.HasExtension {
		length += h.Extension.calcDataLength()
	}
	return
}

func (h *OptionalHeaderExtension) calcDataLength() (length int) {
	length++
	length += 16 * int(util.B2U(h.HasPrivateData))
	if h.HasPackHeaderField {
		length += 1 + len(h.PackHeader)
	}
	length += 2 * int(util.B2U(h.HasProgramPacketSequenceCounter))
	length += 2 * int(util.B2U(h.HasPSTDBuffer))
	if h.HasExtension2 {
		length += 1 + h.extension2FieldLength()
	}
	return
}

func (h *OptionalHeader) putBytes(bs []byte) (n int) {
	if h == nil {
		return 0
	}

	b := uint8(optionalHeaderMarker) << 6
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
	// Header.putBytes refuses a longer header, so the length still fits a byte.
	bs[2] = uint8(h.calcDataLength())
	n = optionalHeaderFixedLen

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

func (h *OptionalHeaderExtension) extension2FieldLength() (n int) {
	n = 1 + len(h.Extension2Reserved)
	if !h.HasStreamIDExtension && h.HasTREF {
		n += ts.PTSDTSSize
	}
	return
}

func (h *OptionalHeaderExtension) putBytes(bs []byte) (n int) {
	bs[0] = util.B2U(h.HasPrivateData) << 7
	bs[0] |= util.B2U(h.HasPackHeaderField) << 6
	bs[0] |= util.B2U(h.HasProgramPacketSequenceCounter) << 5
	bs[0] |= util.B2U(h.HasPSTDBuffer) << 4
	bs[0] |= 0xe
	bs[0] |= util.B2U(h.HasExtension2)
	n = 1

	if h.HasPrivateData {
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
		bs[n] = 0x80 | uint8(h.extension2FieldLength())
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
