package astits

import (
	"errors"
	"fmt"
	"github.com/asticode/go-astikit"
	"sync"
	"unsafe"
)

// Scrambling Controls
const (
	ScramblingControlNotScrambled         = 0
	ScramblingControlReservedForFutureUse = 1
	ScramblingControlScrambledWithEvenKey = 2
	ScramblingControlScrambledWithOddKey  = 3
)

const (
	MpegTsPacketSize       = 188
	mpegTsPacketHeaderSize = 4
	pcrBytesSize           = 6
)

var PoolOfPacket = sync.Pool{
	New: func() interface{} {
		return &Packet{bs: make([]byte, 188)}
	},
}

var errSkippedPacket = errors.New("astits: skipped packet")

// Packet represents a packet
// https://en.wikipedia.org/wiki/MPEG_transport_stream
type Packet struct {
	AdaptationField *PacketAdaptationField
	Header          PacketHeader
	Payload         []byte // This is only the payload content

	bs []byte

	next *Packet
	prev *Packet
}

// PacketHeader represents a packet header
type PacketHeader struct {
	ContinuityCounter          uint8 // Sequence number of payload packets (0x00 to 0x0F) within each stream (except PID 8191)
	HasAdaptationField         bool
	HasPayload                 bool
	PayloadUnitStartIndicator  bool   // Set when a PES, PSI, or DVB-MIP packet begins immediately following the header.
	PID                        uint16 // Packet Identifier, describing the payload data.
	TransportErrorIndicator    bool   // Set when a demodulator can't correct errors from FEC data; indicating the packet is corrupt.
	TransportPriority          bool   // Set when the current packet has a higher priority than other packets with the same PID.
	TransportScramblingControl uint8
}

// PacketAdaptationField represents a packet adaptation field
type PacketAdaptationField struct {
	AdaptationExtensionField          *PacketAdaptationExtensionField
	OPCR                              *ClockReference // Original Program clock reference. Helps when one TS is copied into another
	PCR                               *ClockReference // Program clock reference
	TransportPrivateData              []byte
	TransportPrivateDataLength        int
	Length                            int
	StuffingLength                    int  // Only used in writePacketAdaptationField to request stuffing
	SpliceCountdown                   int  // Indicates how many TS packets from this one a splicing point occurs (Two's complement signed; may be negative)
	IsOneByteStuffing                 bool // Only used for one byte stuffing - if true, adaptation field will be written as one uint8(0). Not part of TS format
	RandomAccessIndicator             bool // Set when the stream may be decoded without errors from this point
	DiscontinuityIndicator            bool // Set if current TS packet is in a discontinuity state with respect to either the continuity counter or the program clock reference
	ElementaryStreamPriorityIndicator bool // Set when this stream should be considered "high priority"
	HasAdaptationExtensionField       bool
	HasOPCR                           bool
	HasPCR                            bool
	HasTransportPrivateData           bool
	HasSplicingCountdown              bool
}

// PacketAdaptationExtensionField represents a packet adaptation extension field
type PacketAdaptationExtensionField struct {
	DTSNextAccessUnit      *ClockReference // The PES DTS of the splice point. Split up as 3 bits, 1 marker bit (0x1), 15 bits, 1 marker bit, 15 bits, and 1 marker bit, for 33 data bits total.
	HasLegalTimeWindow     bool
	HasPiecewiseRate       bool
	HasSeamlessSplice      bool
	LegalTimeWindowIsValid bool
	LegalTimeWindowOffset  uint16 // Extra information for rebroadcasters to determine the state of buffers when packets may be missing.
	Length                 int
	PiecewiseRate          uint32 // The rate of the stream, measured in 188-byte packets, to define the end-time of the LTW.
	SpliceType             uint8  // Indicates the parameters of the H.262 splice.
}

func NewPacket() *Packet {
	return PoolOfPacket.Get().(*Packet)
}

func (p *Packet) Close() {
	p.Reset()
	PoolOfPacket.Put(p)
}

func (p *Packet) Reset() {
	*p = Packet{bs: p.bs[:MpegTsPacketSize]}
}

// parsePacket parses a packet
func (p *Packet) parsePacket(i *astikit.BytesIterator, s PacketSkipper) (err error) {
	// Get next byte
	var b byte
	if b, err = i.NextByte(); err != nil || b != syncByte {
		if err != nil {
			return fmt.Errorf("astits: getting next byte failed: %w", err)
		}
		return ErrPacketMustStartWithASyncByte
	}

	// In case packet size is bigger than 188 bytes, we don't care for the first bytes
	i.Seek(i.Len() - MpegTsPacketSize + 1)
	offsetStart := i.Offset()

	// Parse header
	if err = p.Header.parsePacketHeader(i); err != nil {
		return fmt.Errorf("astits: parsing packet header failed: %w", err)
	}

	// Skip packet
	if s(p) {
		return errSkippedPacket
	}

	// Parse adaptation field
	if p.Header.HasAdaptationField {
		if p.AdaptationField, err = parsePacketAdaptationField(i); err != nil {
			return fmt.Errorf("astits: parsing packet adaptation field failed: %w", err)
		}
	}

	// Build payload
	if p.Header.HasPayload {
		i.Seek(p.payloadOffset(offsetStart))
		if p.Payload, err = i.NextBytesNoCopy(i.Len() - i.Offset()); err != nil {
			return fmt.Errorf("astits: fetching next bytes failed: %w", err)
		}
	}
	return
}

// payloadOffset returns the payload offset
func (p *Packet) payloadOffset(offsetStart int) (offset int) {
	offset = offsetStart + 3
	if p.Header.HasAdaptationField {
		offset += 1 + p.AdaptationField.Length
	}
	return
}

// parsePacketHeader parses the packet header
func (ph *PacketHeader) parsePacketHeader(i *astikit.BytesIterator) (err error) {
	// Get next bytes
	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	{
		ph.ContinuityCounter = bs[2] & 0xf
		ph.HasAdaptationField = bs[2]&0x20 > 0
		ph.HasPayload = bs[2]&0x10 > 0
		ph.PayloadUnitStartIndicator = bs[0]&0x40 > 0
		ph.PID = uint16(bs[0]&0x1f)<<8 | uint16(bs[1])
		ph.TransportErrorIndicator = bs[0]&0x80 > 0
		ph.TransportPriority = bs[0]&0x20 > 0
		ph.TransportScramblingControl = bs[2] >> 6 & 0x3
	}

	return
}

// parsePacketAdaptationField parses the packet adaptation field
func parsePacketAdaptationField(i *astikit.BytesIterator) (a *PacketAdaptationField, err error) {
	// Create adaptation field
	a = &PacketAdaptationField{}

	// Get next byte
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Length
	a.Length = int(b)

	afStartOffset := i.Offset()

	// Valid length
	if a.Length > 0 {
		// Get next byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		// Flags
		a.DiscontinuityIndicator = b&0x80 > 0
		a.RandomAccessIndicator = b&0x40 > 0
		a.ElementaryStreamPriorityIndicator = b&0x20 > 0
		a.HasPCR = b&0x10 > 0
		a.HasOPCR = b&0x08 > 0
		a.HasSplicingCountdown = b&0x04 > 0
		a.HasTransportPrivateData = b&0x02 > 0
		a.HasAdaptationExtensionField = b&0x01 > 0

		// PCR
		if a.HasPCR {
			if a.PCR, err = parsePCR(i); err != nil {
				err = fmt.Errorf("astits: parsing PCR failed: %w", err)
				return
			}
		}

		// OPCR
		if a.HasOPCR {
			if a.OPCR, err = parsePCR(i); err != nil {
				err = fmt.Errorf("astits: parsing PCR failed: %w", err)
				return
			}
		}

		// Splicing countdown
		if a.HasSplicingCountdown {
			if b, err = i.NextByte(); err != nil {
				err = fmt.Errorf("astits: fetching next byte failed: %w", err)
				return
			}
			a.SpliceCountdown = int(b)
		}

		// Transport private data
		if a.HasTransportPrivateData {
			// Length
			if b, err = i.NextByte(); err != nil {
				err = fmt.Errorf("astits: fetching next byte failed: %w", err)
				return
			}
			a.TransportPrivateDataLength = int(b)

			// Data
			if a.TransportPrivateDataLength > 0 {
				if a.TransportPrivateData, err = i.NextBytesNoCopy(a.TransportPrivateDataLength); err != nil {
					err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
					return
				}
			}
		}

		// Adaptation extension
		if a.HasAdaptationExtensionField {
			// Create extension field
			a.AdaptationExtensionField = &PacketAdaptationExtensionField{}

			// Get next byte
			if b, err = i.NextByte(); err != nil {
				err = fmt.Errorf("astits: fetching next byte failed: %w", err)
				return
			}

			// Length
			a.AdaptationExtensionField.Length = int(b)
			if a.AdaptationExtensionField.Length > 0 {
				// Get next byte
				if b, err = i.NextByte(); err != nil {
					err = fmt.Errorf("astits: fetching next byte failed: %w", err)
					return
				}

				// Basic
				a.AdaptationExtensionField.HasLegalTimeWindow = b&0x80 > 0
				a.AdaptationExtensionField.HasPiecewiseRate = b&0x40 > 0
				a.AdaptationExtensionField.HasSeamlessSplice = b&0x20 > 0

				// Legal time window
				if a.AdaptationExtensionField.HasLegalTimeWindow {
					var bs []byte
					if bs, err = i.NextBytesNoCopy(2); err != nil {
						err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
						return
					}
					a.AdaptationExtensionField.LegalTimeWindowIsValid = bs[0]&0x80 > 0
					a.AdaptationExtensionField.LegalTimeWindowOffset = uint16(bs[0]&0x7f)<<8 | uint16(bs[1])
				}

				// Piecewise rate
				if a.AdaptationExtensionField.HasPiecewiseRate {
					var bs []byte
					if bs, err = i.NextBytesNoCopy(3); err != nil {
						err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
						return
					}
					a.AdaptationExtensionField.PiecewiseRate = uint32(bs[0]&0x3f)<<16 | uint32(bs[1])<<8 | uint32(bs[2])
				}

				// Seamless splice
				if a.AdaptationExtensionField.HasSeamlessSplice {
					// Get next byte
					if b, err = i.NextByte(); err != nil {
						err = fmt.Errorf("astits: fetching next byte failed: %w", err)
						return
					}

					// Splice type
					a.AdaptationExtensionField.SpliceType = uint8(b&0xf0) >> 4

					// We need to rewind since the current byte is used by the DTS next access unit as well
					i.Skip(-1)

					// DTS Next access unit
					if a.AdaptationExtensionField.DTSNextAccessUnit, err = parsePTSOrDTS(i); err != nil {
						err = fmt.Errorf("astits: parsing DTS failed: %w", err)
						return
					}
				}
			}
		}
	}

	a.StuffingLength = a.Length - (i.Offset() - afStartOffset)

	return
}

// parsePCR parses a Program Clock Reference
// Program clock reference, stored as 33 bits base, 6 bits reserved, 9 bits extension.
func parsePCR(i *astikit.BytesIterator) (cr *ClockReference, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(6); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	pcr := uint64(bs[0])<<40 | uint64(bs[1])<<32 | uint64(bs[2])<<24 | uint64(bs[3])<<16 | uint64(bs[4])<<8 | uint64(bs[5])
	cr = newClockReference(int64(pcr>>15), int64(pcr&0x1ff))
	return
}

func (p *Packet) writePacket(w *astikit.BitsWriter, bb *[8]byte, targetPacketSize int) (written int, retErr error) {
	n, retErr := writePacketHeader(w, bb, p.Header)
	if retErr != nil {
		return
	}
	written += n

	if p.Header.HasAdaptationField {
		n, retErr = writePacketAdaptationField(w, bb, p.AdaptationField)
		if retErr != nil {
			return
		}
		written += n
	}

	if targetPacketSize-written < len(p.Payload) {
		return 0, fmt.Errorf(
			"writePacket: can't write %d bytes of payload: only %d is available",
			len(p.Payload),
			targetPacketSize-written,
		)
	}

	if p.Header.HasPayload {
		retErr = w.Write(p.Payload)
		if retErr != nil {
			return
		}
		written += len(p.Payload)
	}

	for written < targetPacketSize {
		if retErr = w.Write(uint8(0xff)); retErr != nil {
			return
		}
		written++
	}

	return written, nil
}

var writeBufferPool = sync.Pool{
	New: func() interface{} {
		return new([8]byte)
	},
}

func b2u(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

func writePacketHeader(w *astikit.BitsWriter, bb *[8]byte, h PacketHeader) (written int, retErr error) {
	bb[0] = syncByte
	bb[1] = b2u(h.TransportErrorIndicator) << 7
	bb[1] |= b2u(h.PayloadUnitStartIndicator) << 6
	bb[1] |= b2u(h.TransportPriority) << 5
	bb[1] |= uint8(h.PID >> 8)
	bb[2] = uint8(h.PID)
	bb[3] = h.TransportScramblingControl << 6
	bb[3] |= b2u(h.HasAdaptationField) << 5
	bb[3] |= b2u(h.HasPayload) << 4
	bb[3] |= h.ContinuityCounter

	return mpegTsPacketHeaderSize, w.Write(bb[:4])
}

func writePCR(w *astikit.BitsWriter, bb *[8]byte, cr *ClockReference) (int, error) {
	bb[5] = uint8(cr.Extension)
	bb[4] = uint8(cr.Extension>>8) | 0x3f<<1 | uint8(cr.Base<<7)
	bb[3] = uint8(cr.Base >> 1)
	bb[2] = uint8(cr.Base >> 9)
	bb[1] = uint8(cr.Base >> 17)
	bb[0] = uint8(cr.Base >> 25)

	return pcrBytesSize, w.Write(bb[:6])
}

func calcPacketAdaptationFieldLength(af *PacketAdaptationField) (length uint8) {
	length++
	if af.HasPCR {
		length += pcrBytesSize
	}
	if af.HasOPCR {
		length += pcrBytesSize
	}
	if af.HasSplicingCountdown {
		length++
	}
	if af.HasTransportPrivateData {
		length += 1 + uint8(len(af.TransportPrivateData))
	}
	if af.HasAdaptationExtensionField {
		length += 1 + calcPacketAdaptationFieldExtensionLength(af.AdaptationExtensionField)
	}
	length += uint8(af.StuffingLength)
	return
}

func writePacketAdaptationField(w *astikit.BitsWriter, bb *[8]byte, af *PacketAdaptationField) (bytesWritten int, retErr error) {
	b := astikit.NewBitsWriterBatch(w)

	if af.IsOneByteStuffing {
		b.Write(uint8(0))
		return 1, nil
	}

	bb[0] = calcPacketAdaptationFieldLength(af)
	bytesWritten++

	bb[1] = b2u(af.DiscontinuityIndicator) << 7
	bb[1] |= b2u(af.RandomAccessIndicator) << 6
	bb[1] |= b2u(af.ElementaryStreamPriorityIndicator) << 5
	bb[1] |= b2u(af.HasPCR) << 4
	bb[1] |= b2u(af.HasOPCR) << 3
	bb[1] |= b2u(af.HasSplicingCountdown) << 2
	bb[1] |= b2u(af.HasTransportPrivateData) << 1
	bb[1] |= b2u(af.HasAdaptationExtensionField)
	b.Write(bb[:2])
	bytesWritten++

	if af.HasPCR {
		n, err := writePCR(w, bb, af.PCR)
		if err != nil {
			return 0, err
		}
		bytesWritten += n
	}

	if af.HasOPCR {
		n, err := writePCR(w, bb, af.OPCR)
		if err != nil {
			return 0, err
		}
		bytesWritten += n
	}

	if af.HasSplicingCountdown {
		b.Write(uint8(af.SpliceCountdown))
		bytesWritten++
	}

	if af.HasTransportPrivateData {
		// we can get length from TransportPrivateData itself, why do we need separate field?
		b.Write(uint8(af.TransportPrivateDataLength))
		bytesWritten++
		if af.TransportPrivateDataLength > 0 {
			b.Write(af.TransportPrivateData)
		}
		bytesWritten += len(af.TransportPrivateData)
	}

	if af.HasAdaptationExtensionField {
		n, err := writePacketAdaptationFieldExtension(w, bb, af.AdaptationExtensionField)
		if err != nil {
			return 0, err
		}
		bytesWritten += n
	}

	if af.StuffingLength > 0 {
		fast := af.StuffingLength / 8
		rem := af.StuffingLength - fast*8
		*(*uint64)(unsafe.Pointer(bb)) = ^uint64(0)
		for i := 0; i < fast; i++ {
			b.Write(bb[:])
			bytesWritten += 8
		}
		b.Write(bb[:rem])
		bytesWritten += rem
	}

	retErr = b.Err()
	return
}

func calcPacketAdaptationFieldExtensionLength(afe *PacketAdaptationExtensionField) (length uint8) {
	length++
	if afe.HasLegalTimeWindow {
		length += 2
	}
	if afe.HasPiecewiseRate {
		length += 3
	}
	if afe.HasSeamlessSplice {
		length += ptsOrDTSByteLength
	}
	return length
}

func writePacketAdaptationFieldExtension(w *astikit.BitsWriter, bb *[8]byte, afe *PacketAdaptationExtensionField) (bytesWritten int, retErr error) {
	b := astikit.NewBitsWriterBatch(w)

	bb[0] = calcPacketAdaptationFieldExtensionLength(afe)
	bytesWritten++

	bb[1] = b2u(afe.HasLegalTimeWindow) << 7
	bb[1] |= b2u(afe.HasPiecewiseRate) << 6
	bb[1] |= b2u(afe.HasSeamlessSplice) << 5
	bb[1] |= 0x1f
	bytesWritten++

	b.Write(bb[:2])

	if afe.HasLegalTimeWindow {
		bb[0] = b2u(afe.LegalTimeWindowIsValid) << 7
		bb[0] |= uint8(afe.LegalTimeWindowOffset >> 8)
		bb[1] = uint8(afe.LegalTimeWindowOffset)
		b.Write(bb[:2])
		bytesWritten += 2
	}

	if afe.HasPiecewiseRate {
		bb[0] = 0xC0
		bb[0] |= uint8(afe.PiecewiseRate >> 16)
		bb[1] = uint8(afe.PiecewiseRate >> 8)
		bb[2] = uint8(afe.PiecewiseRate)
		b.Write(bb[:3])
		bytesWritten += 3
	}

	if afe.HasSeamlessSplice {
		n, err := writePTSOrDTS(w, afe.SpliceType, afe.DTSNextAccessUnit)
		if err != nil {
			return 0, err
		}
		bytesWritten += n
	}

	retErr = b.Err()
	return
}

func newStuffingAdaptationField(bytesToStuff int) *PacketAdaptationField {
	if bytesToStuff == 1 {
		return &PacketAdaptationField{
			IsOneByteStuffing: true,
		}
	}

	return &PacketAdaptationField{
		// one byte for length and one for flags
		StuffingLength: bytesToStuff - 2,
	}
}
