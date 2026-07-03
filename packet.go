package astits

import (
	"encoding/binary"
	"fmt"
	"github.com/asticode/go-astikit"
	"io"
	"sync"
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
	M2TsPacketSize         = 192
	mpegTsPacketHeaderSize = 4
	pcrBytesSize           = 6
)

var poolOfPacket = sync.Pool{
	New: func() interface{} {
		return &Packet{}
	},
}

// Packet represents a packet
// https://en.wikipedia.org/wiki/MPEG_transport_stream
type Packet struct {
	bs [M2TsPacketSize]byte
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
	p.Header.putBytes(p.bs[:])
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

// reset clears everything parse может НЕ перезаписать: указатели/слайсы не должны
// переживать свой пакет, а флаги при af.Length == 0 вообще не парсятся (флагового
// байта нет) — стейловый HasPCR давал фантомные PCR. Значения PCR/OPCR/privBuf при
// сброшенных Has-флагах — мусор: читать их без проверки флага нельзя.
func (af *PacketAdaptationField) reset() {
	af.AdaptationExtensionField = nil
	af.TransportPrivateData = nil
	// Чистим только использованный префикс: privBuf детерминирован (нули за
	// текущим контентом), сравнения/копии не зависят от прошлых пакетов
	for i := uint8(0); i < af.TransportPrivateDataLength; i++ {
		af.privBuf[i] = 0
	}
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

// parse parses a packet from bs. Прямой слайс-парс: на горячем per-packet пути нет
// BytesIterator — его call-обвязка на каждом поле была заметной частью стоимости.
func (p *Packet) parse(bs []byte, s PacketSkipper) (skip bool, err error) {
	if len(bs) < MpegTsPacketSize {
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
	hdr := len(bs) - MpegTsPacketSize + 1
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
		p.af.reset()
		p.AdaptationField = &p.af
		if err = p.af.parseBytes(bs, hdr+3); err != nil {
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

func (af *PacketAdaptationField) parseBytes(bs []byte, o int) (err error) {
	af.Length = bs[o]
	o++
	bodyStart := o

	// Valid length
	if af.Length > 0 {
		if o >= len(bs) {
			return ErrShortPacket
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
			if o+pcrBytesSize > len(bs) {
				return ErrShortPacket
			}
			af.PCR = parsePCRBytes(bs[o:])
			o += pcrBytesSize
		}

		// OPCR
		if af.HasOPCR {
			if o+pcrBytesSize > len(bs) {
				return ErrShortPacket
			}
			af.OPCR = parsePCRBytes(bs[o:])
			o += pcrBytesSize
		}

		// Splicing countdown
		if af.HasSplicingCountdown {
			if o >= len(bs) {
				return ErrShortPacket
			}
			af.SpliceCountdown = bs[o]
			o++
		}

		// Transport private data
		if af.HasTransportPrivateData {
			if o >= len(bs) {
				return ErrShortPacket
			}
			af.TransportPrivateDataLength = bs[o]
			o++
			if af.TransportPrivateDataLength > 0 {
				end := o + int(af.TransportPrivateDataLength)
				if end > len(bs) {
					return ErrShortPacket
				}
				// Копия в privBuf: срез больше не смотрит в буфер чтения и
				// переживает value-copy структуры (с repoint'ом)
				af.TransportPrivateData = af.privBuf[:copy(af.privBuf[:], bs[o:end])]
				o = end
			}
		}

		// Adaptation extension: редкий — остаётся на итераторе
		if af.HasAdaptationExtensionField {
			i := astikit.NewBytesIterator(bs)
			i.Seek(o)
			af.AdaptationExtensionField = &PacketAdaptationExtensionField{}
			if err = af.AdaptationExtensionField.parse(i); err != nil {
				return fmt.Errorf("astits: parsing Extension field failed: %w", err)
			}
			o = i.Offset()
		}
	}

	af.StuffingLength = af.Length - uint8(o-bodyStart)

	return nil
}

// parsePCRBytes parses a Program Clock Reference
// Program clock reference, stored as 33 bits base, 6 bits reserved, 9 bits extension.
func parsePCRBytes(b []byte) ClockReference {
	pcr := uint64(binary.BigEndian.Uint32(b[:4]))<<16 | uint64(binary.BigEndian.Uint32(b[2:6]))
	return newClockReference(pcr>>15, pcr&0x1ff)
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
			if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
				err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
				return
			}
			afe.LegalTimeWindowIsValid = bs[0]&0x80 > 0
			afe.LegalTimeWindowOffset = binary.BigEndian.Uint16(bs) & 0x7fff
		}

		// Piecewise rate
		if afe.HasPiecewiseRate {
			var bs []byte
			if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
				err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
				return
			}
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

// writeBytes собирает пакет целиком в bs (len(bs) = целевой размер пакета) и
// отдаёт его одним Write — вместо пофилдовой записи через BitsWriter.
func (p *Packet) writeBytes(bs []byte, w io.Writer) (written int, err error) {
	p.Header.putBytes(bs[:mpegTsPacketHeaderSize])
	written = mpegTsPacketHeaderSize

	if p.Header.HasAdaptationField {
		var n int
		if n, err = p.AdaptationField.putBytes(bs[written:]); err != nil {
			return 0, err
		}
		written += n
	}

	if len(bs)-written < len(p.Payload) {
		return 0, fmt.Errorf(
			"writePacket: can't write %d bytes of payload: only %d is available",
			len(p.Payload),
			len(bs)-written,
		)
	}

	if p.Header.HasPayload {
		written += copy(bs[written:], p.Payload)
	}

	for i := written; i < len(bs); i++ {
		bs[i] = 0xff
	}

	if _, err = w.Write(bs); err != nil {
		return 0, err
	}
	return len(bs), nil
}

func (ph *PacketHeader) putBytes(bb []byte) {
	var val uint32
	val |= uint32(syncByte) << 24
	val |= uint32(b2u(ph.TransportErrorIndicator)) << 23
	val |= uint32(b2u(ph.PayloadUnitStartIndicator)) << 22
	val |= uint32(b2u(ph.TransportPriority)) << 21
	val |= uint32(ph.PID&0x1fff) << 8
	val |= uint32(ph.TransportScramblingControl&0x3) << 6
	val |= uint32(b2u(ph.HasAdaptationField)) << 5
	val |= uint32(b2u(ph.HasPayload)) << 4
	val |= uint32(ph.ContinuityCounter & 0xf)
	binary.BigEndian.PutUint32(bb[:], val)
}

func (ph *PacketHeader) write(w *astikit.BitsWriter, bb []byte) (int, error) {
	ph.putBytes(bb)
	return mpegTsPacketHeaderSize, w.Write(bb[:4])
}

func (cr *ClockReference) putPCRBytes(bs []byte) int {
	var bb [8]byte
	binary.BigEndian.PutUint64(bb[:], cr.Extension()|cr.Base()<<15|0x7e<<8)
	copy(bs, bb[2:])
	return pcrBytesSize
}

func (cr *ClockReference) putPTSOrDTSBytes(bs []byte, flag uint8) int {
	bs[0] = flag<<4 | uint8(cr.Base()>>29) | 1
	bs[1] = uint8(cr.Base() >> 22)
	bs[2] = uint8(cr.Base()>>14) | 1
	bs[3] = uint8(cr.Base() >> 7)
	bs[4] = uint8(cr.Base()<<1) | 1
	return ptsOrDTSByteLength
}

func (af *PacketAdaptationField) calcLength() (length uint8) {
	length++
	length += pcrBytesSize * b2u(af.HasPCR)
	length += pcrBytesSize * b2u(af.HasOPCR)
	length += b2u(af.HasSplicingCountdown)
	length += (1 + uint8(len(af.TransportPrivateData))) * b2u(af.HasTransportPrivateData)
	length += (1 + af.AdaptationExtensionField.calcLength()) * b2u(af.HasAdaptationExtensionField)
	length += af.StuffingLength
	return
}

// putBytes serializes the adaptation field directly into bs, mirroring the wire
// format of the former BitsWriter path byte for byte.
func (af *PacketAdaptationField) putBytes(bs []byte) (n int, err error) {
	if af.IsOneByteStuffing {
		bs[0] = 0
		return 1, nil
	}

	length := af.calcLength()
	if int(length)+1 > len(bs) {
		return 0, ErrShortPacket
	}

	bs[0] = length
	var flags uint8
	flags |= b2u(af.DiscontinuityIndicator) << 7
	flags |= b2u(af.RandomAccessIndicator) << 6
	flags |= b2u(af.ElementaryStreamPriorityIndicator) << 5
	flags |= b2u(af.HasPCR) << 4
	flags |= b2u(af.HasOPCR) << 3
	flags |= b2u(af.HasSplicingCountdown) << 2
	flags |= b2u(af.HasTransportPrivateData) << 1
	flags |= b2u(af.HasAdaptationExtensionField)
	bs[1] = flags
	n = 2

	if af.HasPCR {
		n += af.PCR.putPCRBytes(bs[n:])
	}

	if af.HasOPCR {
		n += af.OPCR.putPCRBytes(bs[n:])
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
	length += 2 * b2u(afe.HasLegalTimeWindow)
	length += 3 * b2u(afe.HasPiecewiseRate)
	length += ptsOrDTSByteLength * b2u(afe.HasSeamlessSplice)
	return length
}

func (afe *PacketAdaptationExtensionField) putBytes(bs []byte) (n int) {
	bs[0] = afe.calcLength()
	bs[1] = b2u(afe.HasLegalTimeWindow)<<7 | b2u(afe.HasPiecewiseRate)<<6 | b2u(afe.HasSeamlessSplice)<<5 | 0x1f
	n = 2

	if afe.HasLegalTimeWindow {
		bs[n] = b2u(afe.LegalTimeWindowIsValid)<<7 | uint8(afe.LegalTimeWindowOffset>>8)
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
		n += afe.DTSNextAccessUnit.putPTSOrDTSBytes(bs[n:], afe.SpliceType)
	}

	return
}

// stuffingAdaptationField переиспользует scratch-AF муксера: аллокация на каждый
// пакет со стаффингом уходит, reset() гарантирует чистоту между использованиями
func (m *Muxer) stuffingAdaptationField(bytesToStuff int) *PacketAdaptationField {
	m.stuffAF.reset()
	if bytesToStuff == 1 {
		m.stuffAF.IsOneByteStuffing = true
	} else {
		// one byte for length and one for flags
		m.stuffAF.StuffingLength = uint8(bytesToStuff - 2)
	}
	return &m.stuffAF
}
