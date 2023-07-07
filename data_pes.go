package astits

import (
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
	Header *PESHeader
}

// PESHeader represents a packet PES header
type PESHeader struct {
	OptionalHeader *PESOptionalHeader
	PacketLength   uint16 // Specifies the number of bytes remaining in the packet after this field. Can be zero. If the PES packet length is set to zero, the PES packet can be of any length. A value of zero for the PES packet length can be used only when the PES packet payload is a video elementary stream.
	StreamID       uint8  // Examples: Audio streams (0xC0-0xDF), Video streams (0xE0-0xEF)
}

// PESOptionalHeader represents a PES optional header
type PESOptionalHeader struct {
	DSMTrickMode                    *DSMTrickMode
	DTS                             *ClockReference
	PTS                             *ClockReference
	ESCR                            *ClockReference
	Extension2Data                  []byte
	PrivateData                     []byte
	ESRate                          uint32
	CRC                             uint16
	PSTDBufferSize                  uint16
	AdditionalCopyInfo              uint8
	DataAlignmentIndicator          bool // True indicates that the PES packet header is immediately followed by the video start code or audio syncword
	Extension2Length                uint8
	HasAdditionalCopyInfo           bool
	HasCRC                          bool
	HasDSMTrickMode                 bool
	HasESCR                         bool
	HasESRate                       bool
	HasExtension                    bool
	HasExtension2                   bool
	HasOptionalFields               bool
	HasPackHeaderField              bool
	HasPrivateData                  bool
	HasProgramPacketSequenceCounter bool
	HasPSTDBuffer                   bool
	HeaderLength                    uint8
	IsCopyrighted                   bool
	IsOriginal                      bool
	MarkerBits                      uint8
	MPEG1OrMPEG2ID                  uint8
	OriginalStuffingLength          uint8
	PacketSequenceCounter           uint8
	PackField                       uint8
	Priority                        bool
	PSTDBufferScale                 uint8
	PTSDTSIndicator                 uint8
	ScramblingControl               uint8
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
func parsePESData(i *astikit.BytesIterator) (d *PESData, err error) {
	// Create data
	d = &PESData{}

	// Skip first 3 bytes that are there to identify the PES payload
	i.Seek(3)

	// Parse header
	var dataStart, dataEnd int
	if d.Header, dataStart, dataEnd, err = parsePESHeader(i); err != nil {
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
	return streamID != StreamIDPaddingStream && streamID != StreamIDPrivateStream2
}

// parsePESHeader parses a PES header
func parsePESHeader(i *astikit.BytesIterator) (h *PESHeader, dataStart, dataEnd int, err error) {
	// Create header
	h = &PESHeader{}

	// Get next bytes
	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	_ = bs[2]
	h.StreamID = bs[0]
	h.PacketLength = uint16(bs[1])<<8 | uint16(bs[2])

	// Update data end
	if h.PacketLength > 0 {
		dataEnd = i.Offset() + int(h.PacketLength)
	} else {
		dataEnd = i.Len()
	}

	// Optional header
	if hasPESOptionalHeader(h.StreamID) {
		if h.OptionalHeader, dataStart, err = parsePESOptionalHeader(i); err != nil {
			err = fmt.Errorf("astits: parsing PES optional header failed: %w", err)
			return
		}
	} else {
		dataStart = i.Offset()
	}
	return
}

// parsePESOptionalHeader parses a PES optional header
func parsePESOptionalHeader(i *astikit.BytesIterator) (h *PESOptionalHeader, dataStart int, err error) {
	// Create header
	h = &PESOptionalHeader{}

	// Get next byte
	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	_ = bs[2]

	h.MarkerBits = bs[0] >> 6
	h.ScramblingControl = bs[0] >> 4 & 0x3
	h.Priority = uint8(bs[0])&0x8 > 0
	h.DataAlignmentIndicator = uint8(bs[0])&0x4 > 0
	h.IsCopyrighted = uint(bs[0])&0x2 > 0
	h.IsOriginal = uint8(bs[0])&0x1 > 0
	h.PTSDTSIndicator = bs[1] >> 6 & 0x3

	// Flags
	h.HasESCR = uint8(bs[1])&0x20 > 0
	h.HasESRate = uint8(bs[1])&0x10 > 0
	h.HasDSMTrickMode = uint8(bs[1])&0x8 > 0
	h.HasAdditionalCopyInfo = uint8(bs[1])&0x4 > 0
	h.HasCRC = uint8(bs[1])&0x2 > 0
	h.HasExtension = uint8(bs[1])&0x1 > 0

	// Header length
	h.HeaderLength = bs[2]

	// Update data start
	dataStart = i.Offset() + int(h.HeaderLength)

	var b byte

	// PTS/DTS
	if h.PTSDTSIndicator == PTSDTSIndicatorOnlyPTS {
		if h.PTS, err = parsePTSOrDTS(i); err != nil {
			err = fmt.Errorf("astits: parsing PTS failed: %w", err)
			return
		}
	} else if h.PTSDTSIndicator == PTSDTSIndicatorBothPresent {
		if h.PTS, err = parsePTSOrDTS(i); err != nil {
			err = fmt.Errorf("astits: parsing PTS failed: %w", err)
			return
		}
		if h.DTS, err = parsePTSOrDTS(i); err != nil {
			err = fmt.Errorf("astits: parsing PTS failed: %w", err)
			return
		}
	}

	// ESCR
	if h.HasESCR {
		if h.ESCR, err = parseESCR(i); err != nil {
			err = fmt.Errorf("astits: parsing ESCR failed: %w", err)
			return
		}
	}

	// ES rate
	if h.HasESRate {
		if bs, err = i.NextBytesNoCopy(3); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		_ = bs[2]
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
		if bs, err = i.NextBytesNoCopy(2); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		_ = bs[1]
		h.CRC = uint16(bs[0])>>8 | uint16(bs[1])
	}

	// Extension
	if h.HasExtension {
		// Get next byte
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
			if bs, err = i.NextBytesNoCopy(2); err != nil {
				err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
				return
			}
			_ = bs[1]
			h.PacketSequenceCounter = uint8(bs[0]) & 0x7f
			h.MPEG1OrMPEG2ID = bs[1] >> 6 & 0x1
			h.OriginalStuffingLength = uint8(bs[1]) & 0x3f
		}

		// P-STD buffer
		if h.HasPSTDBuffer {
			if bs, err = i.NextBytesNoCopy(2); err != nil {
				err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
				return
			}
			_ = bs[1]
			h.PSTDBufferScale = bs[0] >> 5 & 0x1
			h.PSTDBufferSize = uint16(bs[0])&0x1f<<8 | uint16(bs[1])
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
func parsePTSOrDTS(i *astikit.BytesIterator) (cr *ClockReference, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(5); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	_ = bs[4]
	cr = newClockReference(int64(uint64(bs[0])>>1&0x7<<30|uint64(bs[1])<<22|uint64(bs[2])>>1&0x7f<<15|uint64(bs[3])<<7|uint64(bs[4])>>1&0x7f), 0)
	return
}

// parseESCR parses an ESCR
func parseESCR(i *astikit.BytesIterator) (cr *ClockReference, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(6); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	_ = bs[5]
	escr := uint64(bs[0])>>3&0x7<<39 | uint64(bs[0])&0x3<<37 | uint64(bs[1])<<29 | uint64(bs[2])>>3<<24 | uint64(bs[2])&0x3<<22 | uint64(bs[3])<<14 | uint64(bs[4])>>3<<9 | uint64(bs[4])&0x3<<7 | uint64(bs[5])>>1
	cr = newClockReference(int64(escr>>9), int64(escr&0x1ff))
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
func (h *PESHeader) writePESData(w *astikit.BitsWriter, payloadLeft []byte, isPayloadStart bool, bytesAvailable int) (totalBytesWritten, payloadBytesWritten int, err error) {
	if isPayloadStart {
		bb := writeBufferPool.Get().(*[8]byte)
		defer writeBufferPool.Put(bb)
		var n int
		if n, err = h.write(w, bb, len(payloadLeft)); err != nil {
			return
		}
		totalBytesWritten += n
	}

	payloadBytesWritten = bytesAvailable - totalBytesWritten
	if payloadBytesWritten > len(payloadLeft) {
		payloadBytesWritten = len(payloadLeft)
	}

	err = w.Write(payloadLeft[:payloadBytesWritten])
	if err != nil {
		return
	}

	totalBytesWritten += payloadBytesWritten
	return
}

func (h *PESHeader) write(w *astikit.BitsWriter, bb *[8]byte, payloadSize int) (int, error) {
	bb[0] = 0
	bb[1] = 0
	bb[2] = 1
	bb[3] = h.StreamID

	pesPacketLength := 0

	if !h.IsVideoStream() {
		pesPacketLength = payloadSize
		if hasPESOptionalHeader(h.StreamID) {
			pesPacketLength += int(h.OptionalHeader.calcLength())
		}
		if pesPacketLength > 0xffff {
			pesPacketLength = 0
		}
	}

	bb[4] = uint8(pesPacketLength >> 8)
	bb[5] = uint8(pesPacketLength)
	if err := w.Write(bb[:6]); err != nil {
		return 0, err
	}
	bytesWritten := pesHeaderLength

	if hasPESOptionalHeader(h.StreamID) {
		n, err := h.OptionalHeader.write(w, bb)
		if err != nil {
			return 0, err
		}
		bytesWritten += n
	}

	return bytesWritten, nil
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
		length++
		length += 16 * b2u(h.HasPrivateData)

		// TODO
		// if h.HasPackHeaderField { }

		length += 2 * b2u(h.HasProgramPacketSequenceCounter)
		length += 2 * b2u(h.HasPSTDBuffer)
		length += (1 + uint8(len(h.Extension2Data))) * b2u(h.HasExtension2)
	}

	return
}

func (h *PESOptionalHeader) write(w *astikit.BitsWriter, bb *[8]byte) (int, error) {
	if h == nil {
		return 0, nil
	}

	b := astikit.NewBitsWriterBatch(w)

	bb[0] = uint8(0b10) << 6
	bb[0] |= h.ScramblingControl << 4
	bb[0] |= b2u(h.Priority) << 3
	bb[0] |= b2u(h.DataAlignmentIndicator) << 2
	bb[0] |= b2u(h.IsCopyrighted) << 1
	bb[0] |= b2u(h.IsOriginal)
	bb[1] = h.PTSDTSIndicator << 6
	bb[1] |= b2u(h.HasESCR) << 5
	bb[1] |= b2u(h.HasESRate) << 4
	bb[1] |= b2u(h.HasDSMTrickMode) << 3
	bb[1] |= b2u(h.HasAdditionalCopyInfo) << 2
	//flags[1] |= 0 << 1
	bb[1] |= b2u(h.HasExtension)
	bb[2] = h.calcDataLength()

	if err := w.Write(bb[:3]); err != nil {
		return 0, err
	}

	bytesWritten := 3

	if h.PTSDTSIndicator == PTSDTSIndicatorOnlyPTS {
		n, err := writePTSOrDTS(w, bb, 0b0010, h.PTS)
		if err != nil {
			return 0, err
		}
		bytesWritten += n
	}

	if h.PTSDTSIndicator == PTSDTSIndicatorBothPresent {
		n, err := writePTSOrDTS(w, bb, 0b0011, h.PTS)
		if err != nil {
			return 0, err
		}
		bytesWritten += n

		n, err = writePTSOrDTS(w, bb, 0b0001, h.DTS)
		if err != nil {
			return 0, err
		}
		bytesWritten += n
	}

	if h.HasESCR {
		n, err := writeESCR(w, h.ESCR)
		if err != nil {
			return 0, err
		}
		bytesWritten += n
	}

	if h.HasESRate {
		b.Write(true)
		b.WriteN(h.ESRate, 22)
		b.Write(true)
		bytesWritten += 3
	}

	if h.HasDSMTrickMode {
		n, err := h.DSMTrickMode.writeDSMTrickMode(w)
		if err != nil {
			return 0, err
		}
		bytesWritten += n
	}

	if h.HasAdditionalCopyInfo {
		b.Write(true) // marker_bit
		b.WriteN(h.AdditionalCopyInfo, 7)
		bytesWritten++
	}

	if h.HasCRC {
		// TODO, not supported
	}

	if h.HasExtension {
		// exp 10110001
		// act 10111111
		b.Write(h.HasPrivateData)
		b.Write(false) // TODO pack_header_field_flag, not implemented
		//b.Write(h.HasPackHeaderField)
		b.Write(h.HasProgramPacketSequenceCounter)
		b.Write(h.HasPSTDBuffer)
		b.WriteN(uint8(0xff), 3) // reserved
		b.Write(h.HasExtension2)
		bytesWritten++

		if h.HasPrivateData {
			b.WriteBytesN(h.PrivateData, 16, 0)
			bytesWritten += 16
		}

		if h.HasPackHeaderField {
			// TODO (see parsePESOptionalHeader)
		}

		if h.HasProgramPacketSequenceCounter {
			b.Write(true) // marker_bit
			b.WriteN(h.PacketSequenceCounter, 7)
			b.Write(true) // marker_bit
			b.WriteN(h.MPEG1OrMPEG2ID, 1)
			b.WriteN(h.OriginalStuffingLength, 6)
			bytesWritten += 2
		}

		if h.HasPSTDBuffer {
			b.WriteN(uint8(0b01), 2)
			b.WriteN(h.PSTDBufferScale, 1)
			b.WriteN(h.PSTDBufferSize, 13)
			bytesWritten += 2
		}

		if h.HasExtension2 {
			b.Write(true) // marker_bit
			b.WriteN(uint8(len(h.Extension2Data)), 7)
			b.Write(h.Extension2Data)
			bytesWritten += 1 + len(h.Extension2Data)
		}
	}

	return bytesWritten, b.Err()
}

func (m *DSMTrickMode) writeDSMTrickMode(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	b.WriteN(m.TrickModeControl, 3)
	if m.TrickModeControl == TrickModeControlFastForward || m.TrickModeControl == TrickModeControlFastReverse {
		b.WriteN(m.FieldID, 2)
		b.Write(m.IntraSliceRefresh == 1) // it should be boolean
		b.WriteN(m.FrequencyTruncation, 2)
	} else if m.TrickModeControl == TrickModeControlFreezeFrame {
		b.WriteN(m.FieldID, 2)
		b.WriteN(uint8(0xff), 3) // reserved
	} else if m.TrickModeControl == TrickModeControlSlowMotion || m.TrickModeControl == TrickModeControlSlowReverse {
		b.WriteN(m.RepeatControl, 5)
	} else {
		b.WriteN(uint8(0xff), 5) // reserved
	}

	return dsmTrickModeLength, b.Err()
}

func writeESCR(w *astikit.BitsWriter, cr *ClockReference) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	b.WriteN(uint8(0xff), 2)
	b.WriteN(uint64(cr.Base>>30), 3)
	b.Write(true)
	b.WriteN(uint64(cr.Base>>15), 15)
	b.Write(true)
	b.WriteN(uint64(cr.Base), 15)
	b.Write(true)
	b.WriteN(uint64(cr.Extension), 9)
	b.Write(true)

	return escrLength, b.Err()
}

func writePTSOrDTS(w *astikit.BitsWriter, bb *[8]byte, flag uint8, cr *ClockReference) (bytesWritten int, retErr error) {
	bb[0] = flag<<4 | uint8(cr.Base>>29) | 1
	bb[1] = uint8(cr.Base >> 22)
	bb[2] = uint8(cr.Base>>14) | 1
	bb[3] = uint8(cr.Base >> 7)
	bb[4] = uint8(cr.Base<<1) | 1

	return ptsOrDTSByteLength, w.Write(bb[:5])
}
