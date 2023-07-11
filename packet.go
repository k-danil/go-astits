package astits

import (
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
		return &Packet{}
	},
}

// Packet represents a packet
// https://en.wikipedia.org/wiki/MPEG_transport_stream
type Packet struct {
	bs [MpegTsPacketSize]byte

	Header          PacketHeader
	AdaptationField *PacketAdaptationField
	Payload         []byte // This is only the payload content

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
	OPCR                              ClockReference // Original Program clock reference. Helps when one TS is copied into another
	PCR                               ClockReference // Program clock reference
	TransportPrivateData              []byte
	TransportPrivateDataLength        uint8
	Length                            uint8
	StuffingLength                    uint8 // Only used in writePacketAdaptationField to request stuffing
	SpliceCountdown                   uint8 // Indicates how many TS packets from this one a splicing point occurs (Two's complement signed; may be negative)
	IsOneByteStuffing                 bool  // Only used for one byte stuffing - if true, adaptation field will be written as one uint8(0). Not part of TS format
	DiscontinuityIndicator            bool  // Set if current TS packet is in a discontinuity state with respect to either the continuity counter or the program clock reference
	RandomAccessIndicator             bool  // Set when the stream may be decoded without errors from this point
	ElementaryStreamPriorityIndicator bool  // Set when this stream should be considered "high priority"
	HasPCR                            bool
	HasOPCR                           bool
	HasSplicingCountdown              bool
	HasTransportPrivateData           bool
	HasAdaptationExtensionField       bool
}

// PacketAdaptationExtensionField represents a packet adaptation extension field
type PacketAdaptationExtensionField struct {
	DTSNextAccessUnit      ClockReference // The PES DTS of the splice point. Split up as 3 bits, 1 marker bit (0x1), 15 bits, 1 marker bit, 15 bits, and 1 marker bit, for 33 data bits total.
	PiecewiseRate          uint32         // The rate of the stream, measured in 188-byte packets, to define the end-time of the LTW.
	LegalTimeWindowOffset  uint16         // Extra information for rebroadcasters to determine the state of buffers when packets may be missing.
	LegalTimeWindowIsValid bool
	HasLegalTimeWindow     bool
	HasPiecewiseRate       bool
	HasSeamlessSplice      bool
	Length                 uint8
	SpliceType             uint8 // Indicates the parameters of the H.262 splice.
}

func NewPacket() (p *Packet) {
	p, _ = PoolOfPacket.Get().(*Packet)
	return
}

func (p *Packet) Close() {
	PoolOfPacket.Put(p)
}

func (p *Packet) Reset() {
	*p = Packet{}
}

// parsePacket parses a packet
func (p *Packet) parse(i *astikit.BytesIterator, s PacketSkipper) (bool, error) {
	// Get next byte
	if b, err := i.NextByte(); err != nil || b != syncByte {
		if err != nil {
			return false, fmt.Errorf("astits: getting next byte failed: %w", err)
		}
		return false, ErrPacketMustStartWithASyncByte
	}

	// In case packet size is bigger than 188 bytes, we don't care for the first bytes
	i.Seek(i.Len() - MpegTsPacketSize + 1)
	offsetStart := i.Offset()

	// Parse header
	if err := p.Header.parse(i); err != nil {
		return false, fmt.Errorf("astits: parsing packet header failed: %w", err)
	}

	// Skip packet
	if s(p) {
		return true, nil
	}

	// Parse adaptation field
	if p.Header.HasAdaptationField {
		p.AdaptationField = &PacketAdaptationField{}
		if err := p.AdaptationField.parse(i); err != nil {
			return false, fmt.Errorf("astits: parsing packet adaptation field failed: %w", err)
		}
	}

	// Build payload
	if p.Header.HasPayload {
		i.Seek(p.payloadOffset(offsetStart))
		var err error
		if p.Payload, err = i.NextBytesNoCopy(i.Len() - i.Offset()); err != nil {
			return false, fmt.Errorf("astits: fetching next bytes failed: %w", err)
		}
	}
	return false, nil
}

// payloadOffset returns the payload offset
func (p *Packet) payloadOffset(offsetStart int) (offset int) {
	offset = offsetStart + 3
	if p.Header.HasAdaptationField {
		offset += 1 + int(p.AdaptationField.Length)
	}
	return
}

// parsePacketHeader parses the packet header
func (ph *PacketHeader) parse(i *astikit.BytesIterator) (err error) {
	// Get next bytes
	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	b := bs[2]
	ph.TransportScramblingControl = b >> 6 & 0x3
	ph.HasAdaptationField = b&0x20 > 0
	ph.HasPayload = b&0x10 > 0
	ph.ContinuityCounter = b & 0xf
	b = bs[0]
	ph.TransportErrorIndicator = b&0x80 > 0
	ph.PayloadUnitStartIndicator = b&0x40 > 0
	ph.TransportPriority = b&0x20 > 0
	ph.PID = (uint16(bs[1]) | uint16(bs[0])<<8) & 0x1fff

	return
}

// parsePacketAdaptationField parses the packet adaptation field
func (af *PacketAdaptationField) parse(i *astikit.BytesIterator) (err error) {
	// Get next byte
	var b byte
	if af.Length, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	afStartOffset := i.Offset()

	// Valid length
	if af.Length > 0 {
		// Get next byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		// Flags
		af.DiscontinuityIndicator = b&0x80 > 0
		af.RandomAccessIndicator = b&0x40 > 0
		af.ElementaryStreamPriorityIndicator = b&0x20 > 0
		af.HasPCR = b&0x10 > 0
		af.HasOPCR = b&0x08 > 0
		af.HasSplicingCountdown = b&0x04 > 0
		af.HasTransportPrivateData = b&0x02 > 0
		af.HasAdaptationExtensionField = b&0x01 > 0

		// PCR
		if af.HasPCR {
			if err = af.PCR.parsePCR(i); err != nil {
				err = fmt.Errorf("astits: parsing PCR failed: %w", err)
				return
			}
		}

		// OPCR
		if af.HasOPCR {
			if err = af.OPCR.parsePCR(i); err != nil {
				err = fmt.Errorf("astits: parsing PCR failed: %w", err)
				return
			}
		}

		// Splicing countdown
		if af.HasSplicingCountdown {
			if af.SpliceCountdown, err = i.NextByte(); err != nil {
				err = fmt.Errorf("astits: fetching next byte failed: %w", err)
				return
			}
		}

		// Transport private data
		if af.HasTransportPrivateData {
			// Length
			if af.TransportPrivateDataLength, err = i.NextByte(); err != nil {
				err = fmt.Errorf("astits: fetching next byte failed: %w", err)
				return
			}

			// Data
			if af.TransportPrivateDataLength > 0 {
				if af.TransportPrivateData, err = i.NextBytesNoCopy(int(af.TransportPrivateDataLength)); err != nil {
					err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
					return
				}
			}
		}

		// Adaptation extension
		if af.HasAdaptationExtensionField {
			// Create extension field
			af.AdaptationExtensionField = &PacketAdaptationExtensionField{}
			if err = af.AdaptationExtensionField.parse(i); err != nil {
				err = fmt.Errorf("astits: parsing Extension field failed: %w", err)
				return
			}
		}
	}

	af.StuffingLength = af.Length - uint8(i.Offset()-afStartOffset)

	return
}

func (afe *PacketAdaptationExtensionField) parse(i *astikit.BytesIterator) (err error) {
	// Length
	if afe.Length, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	if afe.Length > 0 {
		var b byte
		// Get next byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		// Basic
		afe.HasLegalTimeWindow = b&0x80 > 0
		afe.HasPiecewiseRate = b&0x40 > 0
		afe.HasSeamlessSplice = b&0x20 > 0

		// Legal time window
		if afe.HasLegalTimeWindow {
			var bs []byte
			if bs, err = i.NextBytesNoCopy(2); err != nil {
				err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
				return
			}
			_ = bs[1]
			afe.LegalTimeWindowIsValid = bs[0]&0x80 > 0
			afe.LegalTimeWindowOffset = uint16(bs[0]&0x7f)<<8 | uint16(bs[1])
		}

		// Piecewise rate
		if afe.HasPiecewiseRate {
			var bs []byte
			if bs, err = i.NextBytesNoCopy(3); err != nil {
				err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
				return
			}
			_ = bs[2]
			afe.PiecewiseRate = uint32(bs[0]&0x3f)<<16 | uint32(bs[1])<<8 | uint32(bs[2])
		}

		// Seamless splice
		if afe.HasSeamlessSplice {
			// Get next byte
			if b, err = i.NextByte(); err != nil {
				err = fmt.Errorf("astits: fetching next byte failed: %w", err)
				return
			}

			// Splice type
			afe.SpliceType = b & 0xf0 >> 4

			// We need to rewind since the current byte is used by the DTS next access unit as well
			i.Skip(-1)

			// DTS Next access unit
			if err = afe.DTSNextAccessUnit.parsePTSOrDTS(i); err != nil {
				err = fmt.Errorf("astits: parsing DTS failed: %w", err)
				return
			}
		}
	}
	return
}

// parsePCR parses a Program Clock Reference
// Program clock reference, stored as 33 bits base, 6 bits reserved, 9 bits extension.
func (cr *ClockReference) parsePCR(i *astikit.BytesIterator) (err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(6); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	_ = bs[5]
	pcr := uint64(bs[0])<<40 | uint64(bs[1])<<32 | uint64(bs[2])<<24 | uint64(bs[3])<<16 | uint64(bs[4])<<8 | uint64(bs[5])
	*cr = newClockReference(pcr>>15, pcr&0x1ff)
	return
}

func (p *Packet) write(w *astikit.BitsWriter, bb *[8]byte, targetPacketSize int) (written int, err error) {
	if written, err = p.Header.write(w, bb); err != nil {
		return
	}

	if p.Header.HasAdaptationField {
		var n int
		if n, err = p.AdaptationField.write(w, bb); err != nil {
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
		if err = w.Write(p.Payload); err != nil {
			return
		}
		written += len(p.Payload)
	}

	for written < targetPacketSize {
		if err = w.Write(uint8(0xff)); err != nil {
			return
		}
		written++
	}

	return
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

func (ph *PacketHeader) write(w *astikit.BitsWriter, bb *[8]byte) (int, error) {
	bb[0] = syncByte
	b := b2u(ph.TransportErrorIndicator) << 7
	b |= b2u(ph.PayloadUnitStartIndicator) << 6
	b |= b2u(ph.TransportPriority) << 5
	bb[1] = b | uint8(ph.PID>>8)
	bb[2] = uint8(ph.PID)
	b = ph.TransportScramblingControl << 6
	b |= b2u(ph.HasAdaptationField) << 5
	b |= b2u(ph.HasPayload) << 4
	bb[3] = b | ph.ContinuityCounter

	return mpegTsPacketHeaderSize, w.Write(bb[:4])
}

func (cr *ClockReference) writePCR(w *astikit.BitsWriter, bb *[8]byte) (int, error) {
	bb[0] = uint8(cr.Base() >> 25)
	bb[1] = uint8(cr.Base() >> 17)
	bb[2] = uint8(cr.Base() >> 9)
	bb[3] = uint8(cr.Base() >> 1)
	bb[4] = uint8(cr.Base()<<7) | 0x7e | uint8(cr.Extension()>>8)
	bb[5] = uint8(cr.Extension())

	return pcrBytesSize, w.Write(bb[:6])
}

func (af *PacketAdaptationField) calcLength() (length uint8) {
	length++
	length += pcrBytesSize * b2u(af.HasPCR)
	length += pcrBytesSize * b2u(af.HasOPCR)
	length += b2u(af.HasSplicingCountdown)
	length += (1 + uint8(len(af.TransportPrivateData))) * b2u(af.HasTransportPrivateData)
	if af.HasAdaptationExtensionField {
		length += 1 + af.AdaptationExtensionField.calcLength()
	}
	length += af.StuffingLength
	return
}

func (af *PacketAdaptationField) write(w *astikit.BitsWriter, bb *[8]byte) (bytesWritten int, retErr error) {
	b := astikit.NewBitsWriterBatch(w)

	if af.IsOneByteStuffing {
		b.Write(uint8(0))
		return 1, nil
	}

	bb[0] = af.calcLength()
	bs := b2u(af.DiscontinuityIndicator) << 7
	bs |= b2u(af.RandomAccessIndicator) << 6
	bs |= b2u(af.ElementaryStreamPriorityIndicator) << 5
	bs |= b2u(af.HasPCR) << 4
	bs |= b2u(af.HasOPCR) << 3
	bs |= b2u(af.HasSplicingCountdown) << 2
	bs |= b2u(af.HasTransportPrivateData) << 1
	bb[1] = bs | b2u(af.HasAdaptationExtensionField)
	b.Write(bb[:2])
	bytesWritten += 2

	if af.HasPCR {
		n, err := af.PCR.writePCR(w, bb)
		if err != nil {
			return 0, err
		}
		bytesWritten += n
	}

	if af.HasOPCR {
		n, err := af.OPCR.writePCR(w, bb)
		if err != nil {
			return 0, err
		}
		bytesWritten += n
	}

	if af.HasSplicingCountdown {
		b.Write(af.SpliceCountdown)
		bytesWritten++
	}

	if af.HasTransportPrivateData {
		// we can get length from TransportPrivateData itself, why do we need separate field?
		b.Write(af.TransportPrivateDataLength)
		bytesWritten++
		if af.TransportPrivateDataLength > 0 {
			b.Write(af.TransportPrivateData)
			bytesWritten += len(af.TransportPrivateData)
		}
	}

	if af.HasAdaptationExtensionField {
		n, err := af.AdaptationExtensionField.write(w, bb)
		if err != nil {
			return 0, err
		}
		bytesWritten += n
	}

	if af.StuffingLength > 0 {
		fast := af.StuffingLength / 8
		rem := af.StuffingLength - fast*8
		*(*uint64)(unsafe.Pointer(bb)) = ^uint64(0)
		for i := uint8(0); i < fast; i++ {
			b.Write(bb[:])
			bytesWritten += 8
		}
		b.Write(bb[:rem])
		bytesWritten += int(rem)
	}

	retErr = b.Err()
	return
}

func (afe *PacketAdaptationExtensionField) calcLength() (length uint8) {
	length++
	length += 2 * b2u(afe.HasLegalTimeWindow)
	length += 3 * b2u(afe.HasPiecewiseRate)
	length += ptsOrDTSByteLength * b2u(afe.HasSeamlessSplice)
	return length
}

func (afe *PacketAdaptationExtensionField) write(w *astikit.BitsWriter, bb *[8]byte) (bytesWritten int, retErr error) {
	bb[0] = afe.calcLength()
	bb[1] = b2u(afe.HasLegalTimeWindow) << 7
	bb[1] |= b2u(afe.HasPiecewiseRate) << 6
	bb[1] |= b2u(afe.HasSeamlessSplice) << 5
	bb[1] |= 0x1f
	bytesWritten += 2

	if afe.HasLegalTimeWindow {
		i := bytesWritten
		bb[i] = b2u(afe.LegalTimeWindowIsValid) << 7
		bb[i] |= uint8(afe.LegalTimeWindowOffset >> 8)
		bb[i+1] = uint8(afe.LegalTimeWindowOffset)
		bytesWritten += 2
	}

	if afe.HasPiecewiseRate {
		i := bytesWritten
		bb[i] = 0xC0
		bb[i] |= uint8(afe.PiecewiseRate >> 16)
		bb[i+1] = uint8(afe.PiecewiseRate >> 8)
		bb[i+2] = uint8(afe.PiecewiseRate)
		bytesWritten += 3
	}

	if retErr = w.Write(bb[:bytesWritten]); retErr != nil {
		return 0, retErr
	}

	if afe.HasSeamlessSplice {
		n, err := afe.DTSNextAccessUnit.writePTSOrDTS(w, bb, afe.SpliceType)
		if err != nil {
			return 0, err
		}
		bytesWritten += n
	}

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
		StuffingLength: uint8(bytesToStuff - 2),
	}
}
