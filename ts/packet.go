package ts

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/k-danil/go-astits/v3/internal/util"
)

type ScramblingControl uint8

const (
	ScramblingControlNotScrambled         ScramblingControl = 0
	ScramblingControlReservedForFutureUse ScramblingControl = 1
	ScramblingControlScrambledWithEvenKey ScramblingControl = 2
	ScramblingControlScrambledWithOddKey  ScramblingControl = 3
)

var scramblingControlNames = map[ScramblingControl]string{
	ScramblingControlNotScrambled:         "not_scrambled",
	ScramblingControlReservedForFutureUse: "reserved_for_future_use",
	ScramblingControlScrambledWithEvenKey: "scrambled_with_even_key",
	ScramblingControlScrambledWithOddKey:  "scrambled_with_odd_key",
}

func (c ScramblingControl) String() (s string) {
	var ok bool
	if s, ok = scramblingControlNames[c]; !ok {
		s = fmt.Sprintf("0x%02x", uint8(c))
	}
	return
}

func (c ScramblingControl) MarshalJSON() (b []byte, err error) {
	return json.Marshal(c.String())
}

func (c *ScramblingControl) UnmarshalJSON(b []byte) (err error) {
	*c, err = util.UnmarshalEnum(b, scramblingControlNames)
	return
}

const (
	PacketSize     = 188
	M2TSPacketSize = 192 // 4-byte TP_extra_header prefix + 188
	RSPacketSize   = 204 // 188 + 16-byte Reed-Solomon parity suffix
	HeaderSize     = 4

	m2tsPrefixSize           = M2TSPacketSize - PacketSize
	maxAdaptationFieldLength = 0xff
)

const syncByte byte = '\x47'

var poolOfPacket = sync.Pool{
	New: func() any {
		return &Packet{}
	},
}

type Packet struct {
	bs  [RSPacketSize]byte
	raw []byte

	Header          PacketHeader          `json:"_header"`
	PrefixLen       uint8                 `json:"_prefix_len"`      // 4 for M2TS, 0 otherwise; the sync byte starts at this offset in Raw.
	Prefix          uint32                `json:"_prefix"`          // M2TS TP_extra_header; meaningful only when PrefixLen is 4
	AdaptationField PacketAdaptationField `json:"adaptation_field"` // meaningful only when Header.HasAdaptationField
	Payload         []byte                `json:"data_byte"`

	// Byte offset of the packet start (M2TS prefix included) from the Demuxer's first packet; packets dropped by a skipper advance it too.
	Offset int64 `json:"_offset"`
	// reader's stamp for the window (tsio.Tagger); 0 when untagged
	Tag uint64 `json:"_tag"`
}

// Mutating Header does not touch the packet bytes; call this to write them back.
func (p *Packet) UpdateHeader() {
	bs := p.raw
	if bs == nil {
		bs = p.bs[:]
	}
	p.Header.Put(bs[p.PrefixLen:])
}

type PacketHeader struct {
	ContinuityCounter          uint8             `json:"continuity_counter"`
	HasAdaptationField         bool              `json:"_has_adaptation_field"`
	HasPayload                 bool              `json:"_has_payload"`
	PayloadUnitStartIndicator  bool              `json:"payload_unit_start_indicator"`
	PID                        uint16            `json:"PID"`
	TransportErrorIndicator    bool              `json:"transport_error_indicator"`
	TransportPriority          bool              `json:"transport_priority"`
	TransportScramblingControl ScramblingControl `json:"transport_scrambling_control"`
}

type PacketAdaptationField struct {
	AdaptationExtensionField          *PacketAdaptationExtensionField `json:"adaptation_field_extension"`
	OPCR                              ClockReference                  `json:"OPCR"`
	PCR                               ClockReference                  `json:"PCR"`
	TransportPrivateData              []byte                          `json:"private_data_byte"`             // a view into the packet buffer after parse; CopyFrom takes an owned copy
	TransportPrivateDataLength        uint8                           `json:"transport_private_data_length"` // parse output; Put writes len(TransportPrivateData)
	Length                            uint8                           `json:"adaptation_field_length"`
	StuffingLength                    uint8                           `json:"_stuffing_length"`
	SpliceCountdown                   int8                            `json:"splice_countdown"`
	IsOneByteStuffing                 bool                            `json:"_is_one_byte_stuffing"`
	DiscontinuityIndicator            bool                            `json:"discontinuity_indicator"`
	RandomAccessIndicator             bool                            `json:"random_access_indicator"`
	ElementaryStreamPriorityIndicator bool                            `json:"elementary_stream_priority_indicator"`
	HasPCR                            bool                            `json:"PCR_flag"`
	HasOPCR                           bool                            `json:"OPCR_flag"`
	HasSplicingCountdown              bool                            `json:"splicing_point_flag"`
	HasTransportPrivateData           bool                            `json:"transport_private_data_flag"`
	HasAdaptationExtensionField       bool                            `json:"adaptation_field_extension_flag"`
}

// Parse leaves the flags untouched when af.Length == 0, so a stale HasPCR would yield a phantom PCR.
func (af *PacketAdaptationField) Reset() {
	*af = PacketAdaptationField{}
}

// Takes an owned copy of src. The private-data copy appends into the receiver's own backing, so the receiver's TransportPrivateData must never be a view into a read buffer. The extension struct is shared as is; only its descriptors are copied.
func (af *PacketAdaptationField) CopyFrom(src *PacketAdaptationField) {
	priv := af.TransportPrivateData[:0]
	*af = *src
	if src.TransportPrivateData != nil {
		priv = append(priv, src.TransportPrivateData...)
		af.TransportPrivateData = priv
	}
	if ext := src.AdaptationExtensionField; ext != nil && ext.AFDescriptors != nil {
		own := *ext
		own.AFDescriptors = bytes.Clone(ext.AFDescriptors)
		af.AdaptationExtensionField = &own
	}
}

type PacketAdaptationExtensionField struct {
	DTSNextAccessUnit      ClockReference `json:"DTS_next_AU"`
	AFDescriptors          []byte         `json:"AF_descriptors"` // raw af_descriptor() bytes (H.222.0 Annex U), unparsed
	PiecewiseRate          uint32         `json:"piecewise_rate"` // in 188-byte packets
	LegalTimeWindowOffset  uint16         `json:"ltw_offset"`
	LegalTimeWindowIsValid bool           `json:"ltw_valid_flag"`
	HasLegalTimeWindow     bool           `json:"ltw_flag"`
	HasPiecewiseRate       bool           `json:"piecewise_rate_flag"`
	HasSeamlessSplice      bool           `json:"seamless_splice_flag"`
	HasAFDescriptors       bool           `json:"_has_af_descriptors"` // af_descriptor_not_present_flag == 0
	Length                 uint8          `json:"adaptation_field_extension_length"`
	SpliceType             uint8          `json:"splice_type"`
}

// Pooled; return it with Close.
func NewPacket() (p *Packet) {
	p, _ = poolOfPacket.Get().(*Packet)
	p.Reset()
	return
}

// nil for a hand-built packet. A sync byte repaired under sync lock reads 0x47 here, one byte off the wire.
func (p *Packet) Raw() []byte {
	return p.raw
}

// 2-bit copy_permission_indicator + 30-bit 27 MHz arrival_time_stamp.
func (p *Packet) ArrivalTimeStamp() (copyPermission uint8, ats uint32, ok bool) {
	if p.PrefixLen < m2tsPrefixSize {
		return
	}
	v := p.Prefix
	return uint8(v >> 30), v & 0x3fffffff, true
}

func (p *Packet) SetAdaptationField(src *PacketAdaptationField) {
	p.Header.HasAdaptationField = src != nil
	if src == nil {
		return
	}
	// After a parse the field views the read buffer, and CopyFrom appends into whatever it is given.
	p.AdaptationField.TransportPrivateData = nil
	p.AdaptationField.CopyFrom(src)
}

// Do not use p afterwards.
func (p *Packet) Close() {
	poolOfPacket.Put(p)
}

// bs is left alone: the next read overwrites it in full before any parse.
func (p *Packet) Reset() {
	p.raw = nil
	p.Header = PacketHeader{}
	p.AdaptationField.Reset()
	p.Payload = nil
	p.Prefix = 0
	p.PrefixLen = 0
	p.Offset = 0
	p.Tag = 0
}

// p keeps viewing bs, so it is valid only as long as bs is. A bs of exactly 192 bytes is read as M2TS, prefix included.
func (p *Packet) ParseAt(bs []byte, offset int64, s PacketSkipper, keep *PIDSet) (skip bool, err error) {
	p.Offset = offset
	p.raw = bs
	return p.parse(bs, s, keep)
}

func (p *Packet) parse(bs []byte, s PacketSkipper, keep *PIDSet) (skip bool, err error) {
	if len(bs) < PacketSize {
		return false, ErrShortPacket
	}

	prefixLen := 0
	if len(bs) == M2TSPacketSize {
		prefixLen = m2tsPrefixSize
		p.Prefix = binary.BigEndian.Uint32(bs[:m2tsPrefixSize])
	}
	p.PrefixLen = uint8(prefixLen)

	h := binary.BigEndian.Uint32(bs[prefixLen:])
	if byte(h>>24) != syncByte {
		return false, ErrPacketMustStartWithASyncByte
	}

	p.Header.parseBytes(h)

	if keep != nil && !keep.Has(p.Header.PID) {
		return true, nil
	}
	if s != nil && s(p) {
		return true, nil
	}

	payloadAt := prefixLen + 1 + 3
	end := prefixLen + PacketSize // excludes a trailing RS suffix

	if p.Header.HasAdaptationField {
		p.AdaptationField.Reset()
		var an int
		if an, err = p.AdaptationField.Parse(bs[payloadAt:end]); err != nil {
			return
		}
		payloadAt += an
	}

	// Pooled packets: without the else, Payload keeps the previous packet's bytes.
	if p.Header.HasPayload {
		p.Payload = bs[payloadAt:end]
	} else {
		p.Payload = nil
		// adaptation_field_control '00' is reserved; decoders shall discard (H.222.0 §2.4.3.3).
		if !p.Header.HasAdaptationField {
			return true, ErrReservedAdaptationFieldControl
		}
	}
	return
}

// bs starts at the sync byte.
func (ph *PacketHeader) Parse(bs []byte) (n int, err error) {
	if len(bs) < HeaderSize {
		return 0, ErrShortPacket
	}
	ph.parseBytes(binary.BigEndian.Uint32(bs))
	return HeaderSize, nil
}

// h is the big-endian 4-byte TS header: [sync|b0|b1|b2].
func (ph *PacketHeader) parseBytes(h uint32) {
	b0, b2 := uint8(h>>16), uint8(h)
	ph.TransportErrorIndicator = b0&0x80 > 0
	ph.PayloadUnitStartIndicator = b0&0x40 > 0
	ph.TransportPriority = b0&0x20 > 0
	ph.PID = uint16(h>>8) & 0x1fff
	ph.TransportScramblingControl = ScramblingControl(b2 >> 6 & 0x3)
	ph.HasAdaptationField = b2&0x20 > 0
	ph.HasPayload = b2&0x10 > 0
	ph.ContinuityCounter = b2 & 0xf
}

// bs starts at the adaptation field length byte.
func (af *PacketAdaptationField) Parse(bs []byte) (n int, err error) {
	if len(bs) == 0 {
		return 0, ErrShortPacket
	}
	af.Length = bs[0]
	o := 1
	bodyStart := o
	af.IsOneByteStuffing = af.Length == 0

	if af.Length > 0 {
		if o >= len(bs) {
			return o, ErrShortPacket
		}
		b := bs[o]
		o++

		af.DiscontinuityIndicator = b&0x80 > 0
		af.RandomAccessIndicator = b&0x40 > 0
		af.ElementaryStreamPriorityIndicator = b&0x20 > 0
		af.HasPCR = b&0x10 > 0
		af.HasOPCR = b&0x08 > 0
		af.HasSplicingCountdown = b&0x04 > 0
		af.HasTransportPrivateData = b&0x02 > 0
		af.HasAdaptationExtensionField = b&0x01 > 0

		if af.HasPCR {
			var pn int
			if pn, err = af.PCR.ParsePCR(bs[o:]); err != nil {
				return o, err
			}
			o += pn
		}

		if af.HasOPCR {
			var pn int
			if pn, err = af.OPCR.ParsePCR(bs[o:]); err != nil {
				return o, err
			}
			o += pn
		}

		if af.HasSplicingCountdown {
			if o >= len(bs) {
				return o, ErrShortPacket
			}
			af.SpliceCountdown = int8(bs[o])
			o++
		}

		if af.HasTransportPrivateData {
			if o >= len(bs) {
				return o, ErrShortPacket
			}
			l := bs[o]
			o++
			af.TransportPrivateDataLength = l
			if l > 0 {
				end := o + int(l)
				if end > len(bs) {
					return o, ErrShortPacket
				}
				af.TransportPrivateData = bs[o:end]
				o = end
			}
		}

		if af.HasAdaptationExtensionField {
			af.AdaptationExtensionField = &PacketAdaptationExtensionField{}
			var en int
			if en, err = af.AdaptationExtensionField.Parse(bs[o:]); err != nil {
				return o, fmt.Errorf("astits: parsing Extension field failed: %w", err)
			}
			o += en
		}
	}

	// The field is 1+Length bytes; the body must fit inside it, the rest is stuffing.
	end := bodyStart + int(af.Length)
	if end > len(bs) || end < o {
		return o, ErrShortPacket
	}
	af.StuffingLength = uint8(end - o)

	return end, nil
}

// bs starts at the extension length byte.
func (afe *PacketAdaptationExtensionField) Parse(bs []byte) (n int, err error) {
	if len(bs) == 0 {
		return 0, ErrShortPacket
	}
	afe.Length = bs[0]
	o := 1

	// H.222.0 2.4.3.5: the declared length bounds the fields below, and the flags byte is mandatory, so length 0 is malformed.
	end := 1 + int(afe.Length)
	if afe.Length == 0 || end > len(bs) {
		return o, ErrShortPacket
	}
	bs = bs[:end]

	b := bs[o]
	o++

	afe.HasLegalTimeWindow = b&0x80 > 0
	afe.HasPiecewiseRate = b&0x40 > 0
	afe.HasSeamlessSplice = b&0x20 > 0
	afe.HasAFDescriptors = b&0x10 == 0

	if afe.HasLegalTimeWindow {
		if o+2 > len(bs) {
			return o, ErrShortPacket
		}
		afe.LegalTimeWindowIsValid = bs[o]&0x80 > 0
		afe.LegalTimeWindowOffset = binary.BigEndian.Uint16(bs[o:]) & 0x7fff
		o += 2
	}

	if afe.HasPiecewiseRate {
		if o+3 > len(bs) {
			return o, ErrShortPacket
		}
		afe.PiecewiseRate = uint32(bs[o]&0x3f)<<16 | uint32(bs[o+1])<<8 | uint32(bs[o+2])
		o += 3
	}

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

	if afe.HasAFDescriptors {
		afe.AFDescriptors = bs[o:]
		o = len(bs)
	}
	return o, nil
}

// len(bs) is the target packet size (188); the tail is stuffed with 0xff. An M2TS prefix is not written — pass Raw to keep it.
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

func (ph *PacketHeader) Put(bs []byte) (n int) {
	ph.putBytes(bs)
	return HeaderSize
}

// Patches the CC of a header already written by Put.
func SetContinuityCounter(header []byte, cc uint8) {
	header[HeaderSize-1] = header[HeaderSize-1]&0xf0 | cc&0xf
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
	binary.BigEndian.PutUint32(bb, val)
}

func (af *PacketAdaptationField) carriesFields() bool {
	return af.DiscontinuityIndicator || af.RandomAccessIndicator || af.ElementaryStreamPriorityIndicator ||
		af.HasPCR || af.HasOPCR || af.HasSplicingCountdown || af.HasTransportPrivateData ||
		af.HasAdaptationExtensionField || af.StuffingLength > 0
}

func (af *PacketAdaptationField) CalcLength() (length int) {
	// Length 0 wins over the flags; Put rejects the contradiction.
	if af.IsOneByteStuffing {
		return 0
	}
	length++
	length += PCRSize * int(util.B2U(af.HasPCR))
	length += PCRSize * int(util.B2U(af.HasOPCR))
	length += int(util.B2U(af.HasSplicingCountdown))
	length += (1 + len(af.TransportPrivateData)) * int(util.B2U(af.HasTransportPrivateData))
	length += (1 + af.AdaptationExtensionField.calcLength()) * int(util.B2U(af.HasAdaptationExtensionField))
	length += int(af.StuffingLength)
	return
}

func (af *PacketAdaptationField) Put(bs []byte) (n int, err error) {
	if af.IsOneByteStuffing {
		if af.carriesFields() {
			return 0, ErrContradictoryAdaptationField
		}
		if len(bs) == 0 {
			return 0, ErrShortPacket
		}
		bs[0] = 0
		return 1, nil
	}
	if af.HasAdaptationExtensionField && af.AdaptationExtensionField == nil {
		return 0, ErrContradictoryAdaptationField
	}

	length := af.CalcLength()
	if length > maxAdaptationFieldLength {
		return 0, ErrAdaptationFieldOverflow
	}
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
		bs[n] = uint8(af.SpliceCountdown)
		n++
	}

	if af.HasTransportPrivateData {
		bs[n] = uint8(len(af.TransportPrivateData))
		n++
		n += copy(bs[n:], af.TransportPrivateData)
	}

	if af.HasAdaptationExtensionField {
		n += af.AdaptationExtensionField.putBytes(bs[n:])
	}

	for i := range int(af.StuffingLength) {
		bs[n+i] = 0xff
	}
	n += int(af.StuffingLength)

	return
}

func (afe *PacketAdaptationExtensionField) calcLength() (length int) {
	if afe == nil {
		return 0
	}
	length++
	length += 2 * int(util.B2U(afe.HasLegalTimeWindow))
	length += 3 * int(util.B2U(afe.HasPiecewiseRate))
	length += PTSDTSSize * int(util.B2U(afe.HasSeamlessSplice))
	if afe.HasAFDescriptors {
		length += len(afe.AFDescriptors)
	}
	return
}

func (afe *PacketAdaptationExtensionField) putBytes(bs []byte) (n int) {
	bs[0] = uint8(afe.calcLength())
	bs[1] = util.B2U(afe.HasLegalTimeWindow)<<7 | util.B2U(afe.HasPiecewiseRate)<<6 | util.B2U(afe.HasSeamlessSplice)<<5 | util.B2U(!afe.HasAFDescriptors)<<4 | 0x0f
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

	if afe.HasAFDescriptors {
		n += copy(bs[n:], afe.AFDescriptors)
	}

	return
}
