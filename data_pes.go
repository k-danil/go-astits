package astits

import (
	"encoding/binary"
	"fmt"
	"github.com/asticode/go-astikit"
)

// P-STD buffer scales
const (
	PSTDBufferScale128Bytes  = 0
	PSTDBufferScale1024Bytes = 1
)

// PTS DTS indicator
const (
	PTSDTSIndicatorBothPresent = 3
	PTSDTSIndicatorIsForbidden = 1
	PTSDTSIndicatorNoPTSOrDTS  = 0
	PTSDTSIndicatorOnlyPTS     = 2
)

// Stream IDs
const (
	StreamIDPrivateStream1 = 189
	StreamIDPaddingStream  = 190
	StreamIDPrivateStream2 = 191
)

// Trick mode controls
const (
	TrickModeControlFastForward = 0
	TrickModeControlFastReverse = 3
	TrickModeControlFreezeFrame = 2
	TrickModeControlSlowMotion  = 1
	TrickModeControlSlowReverse = 4
)

const (
	pesHeaderLength    = 6
	ptsOrDTSByteLength = 5
	escrLength         = 6
	dsmTrickModeLength = 1
)

// PESData represents a PES data
// https://en.wikipedia.org/wiki/Packetized_elementary_stream
// http://dvd.sourceforge.net/dvdinfo/pes-hdr.html
// http://happy.emu.id.au/lab/tut/dttb/dtbtut4b.htm
type PESData struct {
	Data   []byte
	Header PESHeader
}

// PESHeader represents a packet PES header
type PESHeader struct {
	OptionalHeader *PESOptionalHeader
	optionalHeader PESOptionalHeader // хранилище для OptionalHeader — без аллокации на PES
	PacketLength   uint16            // Specifies the number of bytes remaining in the packet after this field. Can be zero. If the PES packet length is set to zero, the PES packet can be of any length. A value of zero for the PES packet length can be used only when the PES packet payload is a video elementary stream.
	StreamID       uint8             // Examples: Audio streams (0xC0-0xDF), Video streams (0xE0-0xEF)
}

// PESOptionalHeader represents a PES optional header
type PESOptionalHeader struct {
	DSMTrickMode           *DSMTrickMode
	Extension              *PESOptionalHeaderExtension
	DTS                    ClockReference
	PTS                    ClockReference
	ESCR                   ClockReference
	ESRate                 uint32
	CRC                    uint16
	AdditionalCopyInfo     uint8
	DataAlignmentIndicator bool // True indicates that the PES packet header is immediately followed by the video start code or audio syncword
	HasAdditionalCopyInfo  bool
	HasCRC                 bool
	HasDSMTrickMode        bool
	HasESCR                bool
	HasESRate              bool
	HasExtension           bool
	HasOptionalFields      bool
	HeaderLength           uint8
	IsCopyrighted          bool
	IsOriginal             bool
	MarkerBits             uint8
	Priority               bool
	PTSDTSIndicator        uint8
	ScramblingControl      uint8
}

type PESOptionalHeaderExtension struct {
	PrivateData                     []byte
	Extension2Data                  []byte
	HasPrivateData                  bool
	HasPackHeaderField              bool
	HasProgramPacketSequenceCounter bool
	HasPSTDBuffer                   bool
	HasExtension2                   bool
	PackField                       uint8
	PacketSequenceCounter           uint8
	MPEG1OrMPEG2ID                  uint8
	OriginalStuffingLength          uint8
	PSTDBufferScale                 uint8
	PSTDBufferSize                  uint16
	Extension2Length                uint8
}

// DSMTrickMode represents a DSM trick mode
// https://books.google.fr/books?id=vwUrAwAAQBAJ&pg=PT501&lpg=PT501&dq=dsm+trick+mode+control&source=bl&ots=fI-9IHXMRL&sig=PWnhxrsoMWNQcl1rMCPmJGNO9Ds&hl=fr&sa=X&ved=0ahUKEwjogafD8bjXAhVQ3KQKHeHKD5oQ6AEINDAB#v=onepage&q=dsm%20trick%20mode%20control&f=false
type DSMTrickMode struct {
	FieldID             uint8
	FrequencyTruncation uint8
	IntraSliceRefresh   uint8
	RepeatControl       uint8
	TrickModeControl    uint8
}

func (h *PESHeader) IsVideoStream() bool {
	return h.StreamID == 0xe0 ||
		h.StreamID == 0xfd
}

// parsePESData parses a PES data
func (d *PESData) parsePESData(i *astikit.BytesIterator) (err error) {
	// Skip first 3 bytes that are there to identify the PES payload
	i.Seek(3)

	// Parse header
	var dataStart, dataEnd int
	if dataStart, dataEnd, err = d.Header.parsePESHeader(i); err != nil {
		err = fmt.Errorf("astits: parsing PES header failed: %w", err)
		return
	}

	// Validation
	if dataEnd < dataStart {
		err = fmt.Errorf("astits: data end %d is before data start %d", dataEnd, dataStart)
		return
	}

	// Seek to data
	i.Seek(dataStart)

	// Extract data
	if d.Data, err = i.NextBytesNoCopy(dataEnd - dataStart); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	return
}

// hasPESOptionalHeader checks whether the data has a PES optional header
func hasPESOptionalHeader(streamID uint8) bool {
	return streamID>>1 != 0b1011111
}

// parsePESHeader parses a PES header
func (h *PESHeader) parsePESHeader(i *astikit.BytesIterator) (dataStart, dataEnd int, err error) {
	// Get next bytes
	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	h.StreamID = bs[0]
	h.PacketLength = binary.BigEndian.Uint16(bs[1:])

	// Update data end
	if h.PacketLength > 0 {
		dataEnd = i.Offset() + int(h.PacketLength)
	} else {
		dataEnd = i.Len()
	}

	// Optional header
	if hasPESOptionalHeader(h.StreamID) {
		h.optionalHeader = PESOptionalHeader{}
		h.OptionalHeader = &h.optionalHeader
		if dataStart, err = h.OptionalHeader.parsePESOptionalHeader(i); err != nil {
			err = fmt.Errorf("astits: parsing PES optional header failed: %w", err)
			return
		}
	} else {
		dataStart = i.Offset()
	}
	return
}

// parsePESOptionalHeader parses a PES optional header
func (h *PESOptionalHeader) parsePESOptionalHeader(i *astikit.BytesIterator) (dataStart int, err error) {
	// Get next byte
	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	b := bs[0]
	h.MarkerBits = b >> 6
	h.ScramblingControl = b >> 4 & 0x3
	h.Priority = b&0x8 > 0
	h.DataAlignmentIndicator = b&0x4 > 0
	h.IsCopyrighted = b&0x2 > 0
	h.IsOriginal = b&0x1 > 0
	b = bs[1]
	h.PTSDTSIndicator = b >> 6 & 0x3

	// Flags
	h.HasESCR = b&0x20 > 0
	h.HasESRate = b&0x10 > 0
	h.HasDSMTrickMode = b&0x8 > 0
	h.HasAdditionalCopyInfo = b&0x4 > 0
	h.HasCRC = b&0x2 > 0
	h.HasExtension = b&0x1 > 0

	// Header length
	h.HeaderLength = bs[2]

	// Update data start
	dataStart = i.Offset() + int(h.HeaderLength)

	// PTS/DTS
	if h.PTSDTSIndicator == PTSDTSIndicatorOnlyPTS {
		if err = h.PTS.parsePTSOrDTS(i); err != nil {
			err = fmt.Errorf("astits: parsing PTS failed: %w", err)
			return
		}
	} else if h.PTSDTSIndicator == PTSDTSIndicatorBothPresent {
		if err = h.PTS.parsePTSOrDTS(i); err != nil {
			err = fmt.Errorf("astits: parsing PTS failed: %w", err)
			return
		}
		if err = h.DTS.parsePTSOrDTS(i); err != nil {
			err = fmt.Errorf("astits: parsing PTS failed: %w", err)
			return
		}
	}

	// ESCR
	if h.HasESCR {
		if err = h.ESCR.parseESCR(i); err != nil {
			err = fmt.Errorf("astits: parsing ESCR failed: %w", err)
			return
		}
	}

	// ES rate
	if h.HasESRate {
		if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		h.ESRate = uint32(bs[0])&0x7f<<15 | uint32(bs[1])<<7 | uint32(bs[2])>>1
	}

	// Trick mode
	if h.HasDSMTrickMode {
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		h.DSMTrickMode = parseDSMTrickMode(b)
	}

	// Additional copy info
	if h.HasAdditionalCopyInfo {
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		h.AdditionalCopyInfo = b & 0x7f
	}

	// CRC
	if h.HasCRC {
		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		h.CRC = binary.BigEndian.Uint16(bs)
	}

	// Extension
	if h.HasExtension {
		h.Extension = &PESOptionalHeaderExtension{}
		err = h.Extension.parsePESOptionalHeaderExtension(i)
		return
	}

	return
}

func (h *PESOptionalHeaderExtension) parsePESOptionalHeaderExtension(i *astikit.BytesIterator) (err error) {
	// Get next byte
	var b byte
	var bs []byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Flags
	h.HasPrivateData = b&0x80 > 0
	h.HasPackHeaderField = b&0x40 > 0
	h.HasProgramPacketSequenceCounter = b&0x20 > 0
	h.HasPSTDBuffer = b&0x10 > 0
	h.HasExtension2 = b&0x1 > 0

	// Private data
	if h.HasPrivateData {
		if h.PrivateData, err = i.NextBytesNoCopy(16); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}

	// Pack field length
	if h.HasPackHeaderField {
		if h.PackField, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		// TODO it's only a length of pack_header, should read it all. now it's wrong
	}

	// Program packet sequence counter
	if h.HasProgramPacketSequenceCounter {
		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		h.PacketSequenceCounter = uint8(bs[0]) & 0x7f
		h.MPEG1OrMPEG2ID = bs[1] >> 6 & 0x1
		h.OriginalStuffingLength = uint8(bs[1]) & 0x3f
	}

	// P-STD buffer
	if h.HasPSTDBuffer {
		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		h.PSTDBufferScale = bs[0] >> 5 & 0x1
		h.PSTDBufferSize = binary.BigEndian.Uint16(bs) & 0x1fff
	}

	// Extension 2
	if h.HasExtension2 {
		// Length
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		h.Extension2Length = uint8(b) & 0x7f

		// Data
		if h.Extension2Data, err = i.NextBytesNoCopy(int(h.Extension2Length)); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

// parseDSMTrickMode parses a DSM trick mode
func parseDSMTrickMode(i byte) (m *DSMTrickMode) {
	m = &DSMTrickMode{}
	m.TrickModeControl = i >> 5
	if m.TrickModeControl == TrickModeControlFastForward || m.TrickModeControl == TrickModeControlFastReverse {
		m.FieldID = i >> 3 & 0x3
		m.IntraSliceRefresh = i >> 2 & 0x1
		m.FrequencyTruncation = i & 0x3
	} else if m.TrickModeControl == TrickModeControlFreezeFrame {
		m.FieldID = i >> 3 & 0x3
	} else if m.TrickModeControl == TrickModeControlSlowMotion || m.TrickModeControl == TrickModeControlSlowReverse {
		m.RepeatControl = i & 0x1f
	}
	return
}

// parsePTSOrDTS parses a PTS or a DTS
func (cr *ClockReference) parsePTSOrDTS(i *astikit.BytesIterator) (err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(5); err != nil || len(bs) < 5 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	*cr = newClockReference(uint64(bs[0])>>1&0x7<<30|uint64(bs[1])<<22|uint64(bs[2])>>1&0x7f<<15|uint64(bs[3])<<7|uint64(bs[4])>>1&0x7f, 0)
	return
}

// parseESCR parses an ESCR
func (cr *ClockReference) parseESCR(i *astikit.BytesIterator) (err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(6); err != nil || len(bs) < 6 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	escr := uint64(bs[0])>>3&0x7<<39 | uint64(bs[0])&0x3<<37 | uint64(bs[1])<<29 | uint64(bs[2])>>3<<24 | uint64(bs[2])&0x3<<22 | uint64(bs[3])<<14 | uint64(bs[4])>>3<<9 | uint64(bs[4])&0x3<<7 | uint64(bs[5])>>1
	*cr = newClockReference(escr>>9, escr&0x1ff)
	return
}

// will count how many total bytes and payload bytes will be written when writePESData is called with the same arguments
// should be used by the caller of writePESData to determine AF stuffing size needed to be applied
// since the length of video PES packets are often zero, we can't just stuff it with 0xff-s at the end
func (h *PESHeader) calcPESDataLength(payloadLeft []byte, isPayloadStart bool, bytesAvailable int) (totalBytes, payloadBytes int) {
	totalBytes += pesHeaderLength
	if isPayloadStart {
		totalBytes += int(h.OptionalHeader.calcLength())
	}
	bytesAvailable -= totalBytes

	if len(payloadLeft) < bytesAvailable {
		payloadBytes = len(payloadLeft)
	} else {
		payloadBytes = bytesAvailable
	}

	return
}

// first packet will contain PES header with optional PES header and payload, if possible
// all consequential packets will contain just payload
// for the last packet caller must add AF with stuffing, see calcPESDataLength
func (h *PESHeader) putPESData(bs []byte, payloadLeft []byte, isPayloadStart bool) (totalBytesWritten, payloadBytesWritten int, err error) {
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

func (h *PESHeader) putBytes(bs []byte, payloadSize int) (n int, err error) {
	if len(bs) < pesHeaderLength {
		return 0, ErrShortPacket
	}
	binary.BigEndian.PutUint32(bs, uint32(h.StreamID)|0x1<<8)
	pesPacketLength := 0

	if !h.IsVideoStream() {
		pesPacketLength = payloadSize
		if hasPESOptionalHeader(h.StreamID) {
			pesPacketLength += int(h.OptionalHeader.calcLength())
		}
		pesPacketLength *= int((uint64(pesPacketLength) - 0x10000) >> 63)
	}

	binary.BigEndian.PutUint16(bs[4:], uint16(pesPacketLength))
	n = pesHeaderLength

	if hasPESOptionalHeader(h.StreamID) {
		n += h.OptionalHeader.putBytes(bs[n:])
	}

	return
}

func (h *PESOptionalHeader) calcLength() uint8 {
	if h == nil {
		return 0
	}
	return 3 + h.calcDataLength()
}

func (h *PESOptionalHeader) calcDataLength() (length uint8) {
	if h.PTSDTSIndicator == PTSDTSIndicatorOnlyPTS {
		length += ptsOrDTSByteLength
	} else if h.PTSDTSIndicator == PTSDTSIndicatorBothPresent {
		length += 2 * ptsOrDTSByteLength
	}

	length += escrLength * b2u(h.HasESCR)
	length += 3 * b2u(h.HasESRate)
	length += dsmTrickModeLength * b2u(h.HasDSMTrickMode)
	length += b2u(h.HasAdditionalCopyInfo)

	// TODO
	//if h.HasCRC { length += 4 }

	if h.HasExtension {
		length += h.Extension.calcDataLength()
	}
	return
}

func (h *PESOptionalHeaderExtension) calcDataLength() (length uint8) {
	length++
	length += 16 * b2u(h.HasPrivateData)

	// TODO
	// if h.HasPackHeaderField { }

	length += 2 * b2u(h.HasProgramPacketSequenceCounter)
	length += 2 * b2u(h.HasPSTDBuffer)
	length += (1 + uint8(len(h.Extension2Data))) * b2u(h.HasExtension2)
	return
}

func (h *PESOptionalHeader) putBytes(bs []byte) (n int) {
	if h == nil {
		return 0
	}

	b := uint8(0b10) << 6
	b |= h.ScramblingControl << 4
	b |= b2u(h.Priority) << 3
	b |= b2u(h.DataAlignmentIndicator) << 2
	b |= b2u(h.IsCopyrighted) << 1
	b |= b2u(h.IsOriginal)
	bs[0] = b
	b = h.PTSDTSIndicator << 6
	b |= b2u(h.HasESCR) << 5
	b |= b2u(h.HasESRate) << 4
	b |= b2u(h.HasDSMTrickMode) << 3
	b |= b2u(h.HasAdditionalCopyInfo) << 2
	//flags[1] |= 0 << 1
	b |= b2u(h.HasExtension)
	bs[1] = b
	bs[2] = h.calcDataLength()
	n = 3

	if h.PTSDTSIndicator == PTSDTSIndicatorOnlyPTS {
		n += h.PTS.putPTSOrDTSBytes(bs[n:], 0b0010)
	}

	if h.PTSDTSIndicator == PTSDTSIndicatorBothPresent {
		n += h.PTS.putPTSOrDTSBytes(bs[n:], 0b0011)
		n += h.DTS.putPTSOrDTSBytes(bs[n:], 0b0001)
	}

	if h.HasESCR {
		n += h.ESCR.putESCRBytes(bs[n:])
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

	//if h.HasCRC {
	//	// TODO, not supported
	//}

	if h.HasExtension {
		n += h.Extension.putBytes(bs[n:])
	}

	return
}

func (h *PESOptionalHeaderExtension) putBytes(bs []byte) (n int) {
	// exp 10110001
	// act 10111111
	bs[0] = b2u(h.HasPrivateData) << 7
	//b.Write(false) // TODO pack_header_field_flag, not implemented
	//b.Write(h.HasPackHeaderField)
	bs[0] |= b2u(h.HasProgramPacketSequenceCounter) << 5
	bs[0] |= b2u(h.HasPSTDBuffer) << 4
	bs[0] |= 0xe
	bs[0] |= b2u(h.HasExtension2)
	n = 1

	if h.HasPrivateData {
		// как WriteBytesN: ровно 16 байт, недостающее добивается нулями
		c := copy(bs[n:n+16], h.PrivateData)
		for i := n + c; i < n+16; i++ {
			bs[i] = 0
		}
		n += 16
	}

	//if h.HasPackHeaderField {
	//	// TODO (see parsePESOptionalHeader)
	//}

	if h.HasProgramPacketSequenceCounter {
		bs[n] = 0x80 | h.PacketSequenceCounter
		bs[n+1] = 0x80 | h.MPEG1OrMPEG2ID<<6 | h.OriginalStuffingLength
		n += 2
	}

	if h.HasPSTDBuffer {
		bs[n] = 0x40 | h.PSTDBufferScale<<5 | uint8(h.PSTDBufferSize>>8)
		bs[n+1] = uint8(h.PSTDBufferSize)
		n += 2
	}

	if h.HasExtension2 {
		bs[n] = 0x80 | uint8(len(h.Extension2Data))
		n++
		n += copy(bs[n:], h.Extension2Data)
	}
	return
}

func (m *DSMTrickMode) putBytes(bs []byte) int {
	b := m.TrickModeControl << 5

	if m.TrickModeControl == TrickModeControlFastForward || m.TrickModeControl == TrickModeControlFastReverse {
		b |= m.FieldID<<3 | m.IntraSliceRefresh<<2 | m.FrequencyTruncation
	} else if m.TrickModeControl == TrickModeControlFreezeFrame {
		b |= m.FieldID<<3 | 7
	} else if m.TrickModeControl == TrickModeControlSlowMotion || m.TrickModeControl == TrickModeControlSlowReverse {
		b |= m.RepeatControl
	} else {
		b |= 0x1f
	}

	bs[0] = b
	return dsmTrickModeLength
}

func (cr *ClockReference) putESCRBytes(bs []byte) int {
	bs[0] = 0xc0 | uint8((cr.Base()>>27)&0x38) | 0x04 | uint8((cr.Base()>>28)&0x03)
	bs[1] = uint8(cr.Base() >> 20)
	bs[2] = uint8((cr.Base()>>13)&0x3) | 0x4 | uint8((cr.Base()>>12)&0xf8)
	bs[3] = uint8(cr.Base() >> 5)
	bs[4] = uint8(cr.Extension()>>7) | 0x4 | uint8(cr.Base()<<3)
	bs[5] = uint8(cr.Extension()<<1) | 0x1
	return escrLength
}
