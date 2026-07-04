package ts

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/k-danil/go-astits/v2/internal/util"
)

// Scrambling Controls
const (
	ScramblingControlNotScrambled         = 0
	ScramblingControlReservedForFutureUse = 1
	ScramblingControlScrambledWithEvenKey = 2
	ScramblingControlScrambledWithOddKey  = 3
)

const (
	PacketSize     = 188
	M2TSPacketSize = 192
	HeaderSize     = 4
)

const syncByte byte = '\x47'

var poolOfPacket = sync.Pool{
	New: func() interface{} {
		return &Packet{}
	},
}

// Packet represents a packet
// https://en.wikipedia.org/wiki/MPEG_transport_stream
type Packet struct {
	bs [M2TSPacketSize]byte
	s  uint
	af PacketAdaptationField // AdaptationField points here — no per-packet allocation

	Header          PacketHeader
	AdaptationField *PacketAdaptationField
	Payload         []byte // This is only the payload content

	// Offset is the byte offset of the raw packet start (including any M2TS prefix)
	// within the demuxed stream, counted from the Demuxer's first packet. Packets
	// dropped by a PacketSkipper advance it too, so it stays a valid byte map.
	Offset int64

	next *Packet
}

func (p *Packet) UpdateHeader() {
	p.Header.Put(p.bs[:])
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

// transportPrivateDataMaxSize bounds AF private data: the whole AF is at most 183
// payload bytes.
const transportPrivateDataMaxSize = 184

// PacketAdaptationField represents a packet adaptation field
type PacketAdaptationField struct {
	AdaptationExtensionField          *PacketAdaptationExtensionField
	OPCR                              ClockReference // Original Program clock reference. Helps when one TS is copied into another
	PCR                               ClockReference // Program clock reference
	TransportPrivateData              []byte         // points into privBuf after parse — owned by this struct, survives value copies via repoint
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

	privBuf [transportPrivateDataMaxSize]byte
}

// Reset clears everything parse may NOT overwrite: pointers/slices must not outlive
// their packet, and when af.Length == 0 the flags byte is not parsed at all — a stale
// HasPCR used to produce phantom PCRs. PCR/OPCR/privBuf values behind cleared Has-flags
// are garbage: reading them without checking the flag is forbidden anyway.
func (af *PacketAdaptationField) Reset() {
	af.AdaptationExtensionField = nil
	af.TransportPrivateData = nil
	// Clear only the used prefix: privBuf stays deterministic (zeros beyond the
	// current content), comparisons/copies don't depend on previous packets
	clear(af.privBuf[:af.TransportPrivateDataLength])
	af.TransportPrivateDataLength = 0
	af.Length = 0
	af.StuffingLength = 0
	af.SpliceCountdown = 0
	af.IsOneByteStuffing = false
	af.DiscontinuityIndicator = false
	af.RandomAccessIndicator = false
	af.ElementaryStreamPriorityIndicator = false
	af.HasPCR = false
	af.HasOPCR = false
	af.HasSplicingCountdown = false
	af.HasTransportPrivateData = false
	af.HasAdaptationExtensionField = false
}

// CopyFrom stores an owned copy of src: private data is copied into the
// receiver's own buffer so the field does not alias the source struct.
func (af *PacketAdaptationField) CopyFrom(src *PacketAdaptationField) {
	*af = *src
	if src.TransportPrivateData != nil {
		af.TransportPrivateData = af.privBuf[:copy(af.privBuf[:], src.TransportPrivateData)]
	}
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
	p, _ = poolOfPacket.Get().(*Packet)
	p.Reset()
	return
}

func (p *Packet) Next() *Packet {
	return p.next
}

// Raw returns the raw packet bytes as read from the stream; empty for packets
// that were not read in copy mode (views, hand-built packets).
func (p *Packet) Raw() []byte {
	return p.bs[:p.s]
}

// SetAdaptationField stores an owned copy in the packet's embedded field,
// mirroring post-parse state.
func (p *Packet) SetAdaptationField(src *PacketAdaptationField) {
	if src == nil {
		return
	}
	p.af.CopyFrom(src)
	p.AdaptationField = &p.af
}

func (p *Packet) Close() {
	poolOfPacket.Put(p)
}

// Reset deliberately keeps bs: it is fully overwritten by the next read before any
// parse, and zeroing it per packet doubles the per-packet memory traffic.
func (p *Packet) Reset() {
	p.s = 0
	p.Header = PacketHeader{}
	p.AdaptationField = nil
	p.Payload = nil
	p.Offset = 0
	p.next = nil
}

// parse parses a packet from bs. Direct slice parsing: no BytesIterator on the hot
// per-packet path — its per-field call overhead was a significant share of the cost.
func (p *Packet) parse(bs []byte, s PacketSkipper) (skip bool, err error) {
	if len(bs) < PacketSize {
		return false, ErrShortPacket
	}

	if bs[0] != syncByte {
		err = ErrPacketMustStartWithASyncByte
		// Zero-stuffed packet is skippable; check the actual bytes, not p.bs —
		// in zero-copy mode the packet is a view and p.bs is stale.
		skip = binary.LittleEndian.Uint64(bs[:8]) == 0
		return
	}

	// In case packet size is bigger than 188 bytes, we don't care for the first bytes
	hdr := len(bs) - PacketSize + 1
	p.Header.parseBytes(bs[hdr : hdr+3 : hdr+3])

	// Skip packet
	if s(p) {
		return true, nil
	}

	// A reused packet must not leak the previous packet's fields into one
	// that has no adaptation field / payload of its own.
	p.AdaptationField = nil
	p.Payload = nil

	// Parse adaptation field
	if p.Header.HasAdaptationField {
		p.af.Reset()
		p.AdaptationField = &p.af
		if _, err = p.af.Parse(bs[hdr+3:]); err != nil {
			return
		}
	}

	// Build payload
	if p.Header.HasPayload {
		payloadAt := hdr + 3
		if p.Header.HasAdaptationField {
			payloadAt += 1 + int(p.af.Length)
		}
		if payloadAt > len(bs) {
			return false, ErrShortPacket
		}
		p.Payload = bs[payloadAt:]
	}
	return
}

// Parse parses a 4-byte packet header starting at the sync byte.
func (ph *PacketHeader) Parse(bs []byte) (n int, err error) {
	if len(bs) < HeaderSize {
		return 0, ErrShortPacket
	}
	ph.parseBytes(bs[1:4])
	return HeaderSize, nil
}

func (ph *PacketHeader) parseBytes(b []byte) {
	_ = b[2]
	ph.TransportErrorIndicator = b[0]&0x80 > 0
	ph.PayloadUnitStartIndicator = b[0]&0x40 > 0
	ph.TransportPriority = b[0]&0x20 > 0
	ph.PID = binary.BigEndian.Uint16(b[:2]) & 0x1fff
	ph.TransportScramblingControl = b[2] >> 6 & 0x3
	ph.HasAdaptationField = b[2]&0x20 > 0
	ph.HasPayload = b[2]&0x10 > 0
	ph.ContinuityCounter = b[2] & 0xf
}

// Parse parses an adaptation field starting at its length byte.
func (af *PacketAdaptationField) Parse(bs []byte) (n int, err error) {
	if len(bs) == 0 {
		return 0, ErrShortPacket
	}
	af.Length = bs[0]
	o := 1
	bodyStart := o

	// Valid length
	if af.Length > 0 {
		if o >= len(bs) {
			return o, ErrShortPacket
		}
		b := bs[o]
		o++

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
			var pn int
			if pn, err = af.PCR.ParsePCR(bs[o:]); err != nil {
				return o, err
			}
			o += pn
		}

		// OPCR
		if af.HasOPCR {
			var pn int
			if pn, err = af.OPCR.ParsePCR(bs[o:]); err != nil {
				return o, err
			}
			o += pn
		}

		// Splicing countdown
		if af.HasSplicingCountdown {
			if o >= len(bs) {
				return o, ErrShortPacket
			}
			af.SpliceCountdown = bs[o]
			o++
		}

		// Transport private data
		if af.HasTransportPrivateData {
			if o >= len(bs) {
				return o, ErrShortPacket
			}
			af.TransportPrivateDataLength = bs[o]
			o++
			if af.TransportPrivateDataLength > 0 {
				end := o + int(af.TransportPrivateDataLength)
				if end > len(bs) {
					return o, ErrShortPacket
				}
				// Copy into privBuf: the slice no longer views the read buffer and
				// survives value copies of the struct (with a repoint)
				af.TransportPrivateData = af.privBuf[:copy(af.privBuf[:], bs[o:end])]
				o = end
			}
		}

		// Adaptation extension
		if af.HasAdaptationExtensionField {
			af.AdaptationExtensionField = &PacketAdaptationExtensionField{}
			var en int
			if en, err = af.AdaptationExtensionField.Parse(bs[o:]); err != nil {
				return o, fmt.Errorf("astits: parsing Extension field failed: %w", err)
			}
			o += en
		}
	}

	af.StuffingLength = af.Length - uint8(o-bodyStart)

	return o, nil
}

// Parse parses an adaptation extension field starting at its length byte.
func (afe *PacketAdaptationExtensionField) Parse(bs []byte) (n int, err error) {
	if len(bs) == 0 {
		return 0, ErrShortPacket
	}
	afe.Length = bs[0]
	o := 1

	if afe.Length > 0 {
		if o >= len(bs) {
			return o, ErrShortPacket
		}
		b := bs[o]
		o++

		// Basic
		afe.HasLegalTimeWindow = b&0x80 > 0
		afe.HasPiecewiseRate = b&0x40 > 0
		afe.HasSeamlessSplice = b&0x20 > 0

		// Legal time window
		if afe.HasLegalTimeWindow {
			if o+2 > len(bs) {
				return o, ErrShortPacket
			}
			afe.LegalTimeWindowIsValid = bs[o]&0x80 > 0
			afe.LegalTimeWindowOffset = binary.BigEndian.Uint16(bs[o:]) & 0x7fff
			o += 2
		}

		// Piecewise rate
		if afe.HasPiecewiseRate {
			if o+3 > len(bs) {
				return o, ErrShortPacket
			}
			afe.PiecewiseRate = uint32(bs[o]&0x3f)<<16 | uint32(bs[o+1])<<8 | uint32(bs[o+2])
			o += 3
		}

		// Seamless splice
		if afe.HasSeamlessSplice {
			if o >= len(bs) {
				return o, ErrShortPacket
			}
			// Splice type shares its byte with the DTS next access unit
			afe.SpliceType = bs[o] & 0xf0 >> 4

			var pn int
			if pn, err = afe.DTSNextAccessUnit.ParsePTSDTS(bs[o:]); err != nil {
				err = fmt.Errorf("astits: parsing DTS failed: %w", err)
				return
			}
			o += pn
		}
	}
	return o, nil
}

// Put assembles the whole packet in bs; len(bs) is the target packet size,
// the tail is stuffed with 0xff.
func (p *Packet) Put(bs []byte) (n int, err error) {
	p.Header.Put(bs)
	n = HeaderSize

	if p.Header.HasAdaptationField {
		var an int
		if an, err = p.AdaptationField.Put(bs[n:]); err != nil {
			return 0, err
		}
		n += an
	}

	if len(bs)-n < len(p.Payload) {
		return 0, fmt.Errorf(
			"astits: can't put %d bytes of payload: only %d is available",
			len(p.Payload),
			len(bs)-n,
		)
	}

	if p.Header.HasPayload {
		n += copy(bs[n:], p.Payload)
	}

	for i := n; i < len(bs); i++ {
		bs[i] = 0xff
	}

	return len(bs), nil
}

// Put serializes the 4-byte packet header (including the sync byte) into bs.
func (ph *PacketHeader) Put(bs []byte) (n int) {
	ph.putBytes(bs)
	return HeaderSize
}

func (ph *PacketHeader) putBytes(bb []byte) {
	var val uint32
	val |= uint32(syncByte) << 24
	val |= uint32(util.B2U(ph.TransportErrorIndicator)) << 23
	val |= uint32(util.B2U(ph.PayloadUnitStartIndicator)) << 22
	val |= uint32(util.B2U(ph.TransportPriority)) << 21
	val |= uint32(ph.PID&0x1fff) << 8
	val |= uint32(ph.TransportScramblingControl&0x3) << 6
	val |= uint32(util.B2U(ph.HasAdaptationField)) << 5
	val |= uint32(util.B2U(ph.HasPayload)) << 4
	val |= uint32(ph.ContinuityCounter & 0xf)
	binary.BigEndian.PutUint32(bb[:], val)
}

func (af *PacketAdaptationField) CalcLength() int {
	var length uint8
	length++
	length += PCRSize * util.B2U(af.HasPCR)
	length += PCRSize * util.B2U(af.HasOPCR)
	length += util.B2U(af.HasSplicingCountdown)
	length += (1 + uint8(len(af.TransportPrivateData))) * util.B2U(af.HasTransportPrivateData)
	length += (1 + af.AdaptationExtensionField.calcLength()) * util.B2U(af.HasAdaptationExtensionField)
	length += af.StuffingLength
	return int(length)
}

// Put serializes the adaptation field directly into bs, mirroring the wire
// format of the former BitsWriter path byte for byte.
func (af *PacketAdaptationField) Put(bs []byte) (n int, err error) {
	if af.IsOneByteStuffing {
		bs[0] = 0
		return 1, nil
	}

	length := af.CalcLength()
	if length+1 > len(bs) {
		return 0, ErrShortPacket
	}

	bs[0] = uint8(length)
	var flags uint8
	flags |= util.B2U(af.DiscontinuityIndicator) << 7
	flags |= util.B2U(af.RandomAccessIndicator) << 6
	flags |= util.B2U(af.ElementaryStreamPriorityIndicator) << 5
	flags |= util.B2U(af.HasPCR) << 4
	flags |= util.B2U(af.HasOPCR) << 3
	flags |= util.B2U(af.HasSplicingCountdown) << 2
	flags |= util.B2U(af.HasTransportPrivateData) << 1
	flags |= util.B2U(af.HasAdaptationExtensionField)
	bs[1] = flags
	n = 2

	if af.HasPCR {
		n += af.PCR.PutPCR(bs[n:])
	}

	if af.HasOPCR {
		n += af.OPCR.PutPCR(bs[n:])
	}

	if af.HasSplicingCountdown {
		bs[n] = af.SpliceCountdown
		n++
	}

	if af.HasTransportPrivateData {
		bs[n] = af.TransportPrivateDataLength
		n++
		n += copy(bs[n:], af.TransportPrivateData)
	}

	if af.HasAdaptationExtensionField {
		n += af.AdaptationExtensionField.putBytes(bs[n:])
	}

	for i := 0; i < int(af.StuffingLength); i++ {
		bs[n+i] = 0xff
	}
	n += int(af.StuffingLength)

	return
}

func (afe *PacketAdaptationExtensionField) calcLength() (length uint8) {
	if afe == nil {
		return 0
	}
	length++
	length += 2 * util.B2U(afe.HasLegalTimeWindow)
	length += 3 * util.B2U(afe.HasPiecewiseRate)
	length += PTSDTSSize * util.B2U(afe.HasSeamlessSplice)
	return length
}

func (afe *PacketAdaptationExtensionField) putBytes(bs []byte) (n int) {
	bs[0] = afe.calcLength()
	bs[1] = util.B2U(afe.HasLegalTimeWindow)<<7 | util.B2U(afe.HasPiecewiseRate)<<6 | util.B2U(afe.HasSeamlessSplice)<<5 | 0x1f
	n = 2

	if afe.HasLegalTimeWindow {
		bs[n] = util.B2U(afe.LegalTimeWindowIsValid)<<7 | uint8(afe.LegalTimeWindowOffset>>8)
		bs[n+1] = uint8(afe.LegalTimeWindowOffset)
		n += 2
	}

	if afe.HasPiecewiseRate {
		bs[n] = 0xC0 | uint8(afe.PiecewiseRate>>16)
		bs[n+1] = uint8(afe.PiecewiseRate >> 8)
		bs[n+2] = uint8(afe.PiecewiseRate)
		n += 3
	}

	if afe.HasSeamlessSplice {
		n += afe.DTSNextAccessUnit.PutPTSDTS(bs[n:], afe.SpliceType)
	}

	return
}
