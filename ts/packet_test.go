package ts

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/k-danil/go-astits/v2/internal/bitstest"
)

func packet(h PacketHeader, a *PacketAdaptationField, i []byte, packet192bytes bool) ([]byte, *Packet) {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	var prefix []byte
	if packet192bytes {
		prefix = []byte("test") // M2TS TP_extra_header prefix (4 bytes) before the sync byte
		_ = w.Write(prefix)
	}
	_ = w.Write(syncByte)                                           // Sync byte
	_ = w.Write(packetHeaderBytes(h, "11"))                         // Header
	_ = w.Write(packetAdaptationFieldBytes(a))                      // Adaptation field
	var payload = append(i, bytes.Repeat([]byte{0}, 147-len(i))...) // Payload
	_ = w.Write(payload)
	pk := &Packet{
		Header:  packetHeader,
		Payload: payload,
		Prefix:  prefix,
	}
	pk.af = *packetAdaptationField
	pk.AdaptationField = &pk.af
	return buf.Bytes(), pk
}

func TestParsePacket204(t *testing.T) {
	// A 188-byte TS packet (payload only) plus a 16-byte Reed-Solomon suffix.
	ts := make([]byte, PacketSize)
	ts[0] = syncByte
	ts[1] = 0x40 // payload_unit_start_indicator, PID hi 0
	ts[3] = 0x10 // payload only, CC 0
	copy(ts[4:], []byte("payload"))
	b204 := append(ts, bytes.Repeat([]byte{0xaa}, RSPacketSize-PacketSize)...)

	p := new(Packet)
	_, err := p.parse(b204, EmptySkipper)
	assert.NoError(t, err)
	assert.True(t, p.Header.PayloadUnitStartIndicator)
	assert.Nil(t, p.Prefix)
	assert.Len(t, p.Payload, PacketSize-HeaderSize) // 184; the 16 RS parity bytes are excluded
}

func packetShort(h PacketHeader, payload []byte) ([]byte, *Packet) {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write(syncByte)                   // Sync byte
	_ = w.Write(packetHeaderBytes(h, "01")) // Header
	p := append(payload, bytes.Repeat([]byte{0}, PacketSize-buf.Len())...)
	_ = w.Write(p)
	return buf.Bytes(), &Packet{
		Header:  h,
		Payload: payload,
	}
}

func TestParsePacket(t *testing.T) {
	// Packet not starting with a sync
	bs := make([]byte, PacketSize)
	bs[1] = 1 // Invalid sync byte, not zero-stuffed
	p := new(Packet)
	_, err := p.parse(bs, EmptySkipper)
	assert.EqualError(t, err, ErrPacketMustStartWithASyncByte.Error())

	// Valid
	b, ep := packet(packetHeader, packetAdaptationField, []byte("payload"), true)
	p = new(Packet)
	_, err = p.parse(b, EmptySkipper)
	assert.NoError(t, err)
	assert.Equal(t, p, ep)

	// Skip
	p = new(Packet)
	var skip bool
	skip, err = p.parse(b, func(_ *Packet) (skip bool) { return true })
	assert.NoError(t, err)
	assert.Equal(t, skip, true)
}

//func TestPayloadOffset(t *testing.T) {
//	assert.Equal(t, 3, payloadOffset(0, PacketHeader{}, nil))
//	assert.Equal(t, 7, payloadOffset(1, PacketHeader{HasAdaptationField: true}, &PacketAdaptationField{Length: 2}))
//}

func TestWritePacket(t *testing.T) {
	eb, ep := packet(packetHeader, packetAdaptationField, []byte("payload"), false)
	scratch := make([]byte, PacketSize)
	n, err := ep.Put(scratch)
	assert.NoError(t, err)
	assert.Equal(t, PacketSize, n)
	assert.Equal(t, len(eb), n)
	assert.Equal(t, eb, scratch[:n])
}

func BenchmarkWritePacket(b *testing.B) {
	_, ep := packet(packetHeader, packetAdaptationField, []byte("payload"), false)
	scratch := make([]byte, PacketSize)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = ep.Put(scratch)
	}
}

func TestWritePacket_HeaderOnly(t *testing.T) {
	shortPacketHeader := packetHeader
	shortPacketHeader.HasPayload = false
	shortPacketHeader.HasAdaptationField = false
	_, ep := packetShort(shortPacketHeader, nil)

	scratch := make([]byte, PacketSize)
	n, err := ep.Put(scratch)
	assert.NoError(t, err)
	assert.Equal(t, PacketSize, n)
	buf := bytes.NewBuffer(scratch[:n])

	// we can't just compare bytes returned by packetShort since they're not completely correct,
	//  so we just cross-check writePacket with parsePacket
	p := new(Packet)
	_, err = p.parse(buf.Bytes(), EmptySkipper)
	assert.NoError(t, err)
	assert.Equal(t, ep, p)
}

var packetHeader = PacketHeader{
	ContinuityCounter:          10,
	HasAdaptationField:         true,
	HasPayload:                 true,
	PayloadUnitStartIndicator:  true,
	PID:                        5461,
	TransportErrorIndicator:    true,
	TransportPriority:          true,
	TransportScramblingControl: ScramblingControlScrambledWithEvenKey,
}

func packetHeaderBytes(h PacketHeader, afControl string) []byte {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write(h.TransportErrorIndicator)                // Transport error indicator
	_ = w.Write(h.PayloadUnitStartIndicator)              // Payload unit start indicator
	_ = w.Write("1")                                      // Transport priority
	_ = w.Write(fmt.Sprintf("%.13b", h.PID))              // PID
	_ = w.Write("10")                                     // Scrambling control
	_ = w.Write(afControl)                                // Adaptation field control
	_ = w.Write(fmt.Sprintf("%.4b", h.ContinuityCounter)) // Continuity counter
	return buf.Bytes()
}

func TestParsePacketHeader(t *testing.T) {
	v := PacketHeader{}
	v.parseBytes(packetHeaderBytes(packetHeader, "11"))
	assert.Equal(t, packetHeader, v)
}

func TestWritePacketHeader(t *testing.T) {
	bb := new([8]byte)
	header := append([]byte{syncByte}, packetHeaderBytes(packetHeader, "11")...)
	bytesWritten := packetHeader.Put(bb[:])
	assert.Equal(t, bytesWritten, 4)
	assert.Equal(t, header, bb[:4])
}

func BenchmarkWritePacketHeader(b *testing.B) {
	bb := new([8]byte)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		packetHeader.Put(bb[:])
	}
}

var packetAdaptationField = &PacketAdaptationField{
	AdaptationExtensionField: &PacketAdaptationExtensionField{
		DTSNextAccessUnit:      dtsClockReference,
		HasLegalTimeWindow:     true,
		HasPiecewiseRate:       true,
		HasSeamlessSplice:      true,
		LegalTimeWindowIsValid: true,
		LegalTimeWindowOffset:  10922,
		Length:                 11,
		PiecewiseRate:          2796202,
		SpliceType:             2,
	},
	DiscontinuityIndicator:            true,
	ElementaryStreamPriorityIndicator: true,
	HasAdaptationExtensionField:       true,
	HasOPCR:                           true,
	HasPCR:                            true,
	HasTransportPrivateData:           true,
	HasSplicingCountdown:              true,
	Length:                            36,
	OPCR:                              pcr,
	PCR:                               pcr,
	RandomAccessIndicator:             true,
	SpliceCountdown:                   2,
	TransportPrivateDataLength:        4,
	TransportPrivateData:              []byte("test"),
	StuffingLength:                    5,
}

func packetAdaptationFieldBytes(a *PacketAdaptationField) []byte {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write(uint8(36))                // Length
	_ = w.Write(a.DiscontinuityIndicator) // Discontinuity indicator
	_ = w.Write("1")                      // Random access indicator
	_ = w.Write("1")                      // Elementary stream priority indicator
	_ = w.Write("1")                      // PCR flag
	_ = w.Write("1")                      // OPCR flag
	_ = w.Write("1")                      // Splicing point flag
	_ = w.Write("1")                      // Transport data flag
	_ = w.Write("1")                      // Adaptation field extension flag
	_ = w.Write(pcrBytes())               // PCR
	_ = w.Write(pcrBytes())               // OPCR
	_ = w.Write(uint8(2))                 // Splice countdown
	_ = w.Write(uint8(4))                 // Transport private data length
	_ = w.Write([]byte("test"))           // Transport private data
	_ = w.Write(uint8(11))                // Adaptation extension length
	_ = w.Write("1")                      // LTW flag
	_ = w.Write("1")                      // Piecewise rate flag
	_ = w.Write("1")                      // Seamless splice flag
	_ = w.Write("11111")                  // Reserved
	_ = w.Write("1")                      // LTW valid flag
	_ = w.Write("010101010101010")        // LTW offset
	_ = w.Write("11")                     // Piecewise rate reserved
	_ = w.Write("1010101010101010101010") // Piecewise rate
	_ = w.Write(dtsBytes("0010"))         // Splice type + DTS next access unit
	_ = w.WriteN(^uint64(0), 40)          // Stuffing bytes
	return buf.Bytes()
}

func TestParsePacketAdaptationField(t *testing.T) {
	af := &PacketAdaptationField{}
	_, err := af.Parse(packetAdaptationFieldBytes(packetAdaptationField))
	assert.Equal(t, packetAdaptationField, af)
	assert.NoError(t, err)
}

func TestWritePacketAdaptationField(t *testing.T) {
	eb := packetAdaptationFieldBytes(packetAdaptationField)
	bs := make([]byte, PacketSize)
	bytesWritten, err := packetAdaptationField.Put(bs)
	assert.NoError(t, err)
	assert.Equal(t, len(eb), bytesWritten)
	assert.Equal(t, eb, bs[:bytesWritten])
}

func BenchmarkWritePacketAdaptationField(b *testing.B) {
	bs := make([]byte, PacketSize)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = packetAdaptationField.Put(bs)
	}
}

var pcr = NewClockReference(5726623061, 341)

func pcrBytes() []byte {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write("101010101010101010101010101010101") // Base
	_ = w.Write("111111")                            // Reserved
	_ = w.Write("101010101")                         // Extension
	return buf.Bytes()
}

func TestParsePCR(t *testing.T) {
	var v ClockReference
	n, err := v.ParsePCR(pcrBytes())
	assert.NoError(t, err)
	assert.Equal(t, PCRSize, n)
	assert.Equal(t, pcr, v)
}

func BenchmarkParsePCR(b *testing.B) {
	b.ReportAllocs()

	bs := pcrBytes()

	var v ClockReference
	for i := 0; i < b.N; i++ {
		_, _ = v.ParsePCR(bs)
	}
	_ = v
}

func TestWritePCR(t *testing.T) {
	bs := make([]byte, PCRSize)
	bytesWritten := pcr.PutPCR(bs)
	assert.Equal(t, bytesWritten, 6)
	assert.Equal(t, pcrBytes(), bs)
}

func BenchmarkWritePCR(b *testing.B) {
	bs := make([]byte, PCRSize)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		pcr.PutPCR(bs)
	}
}

func BenchmarkParsePacket(b *testing.B) {
	bs, _ := packet(packetHeader, packetAdaptationField, []byte("payload"), true)

	p := NewPacket()
	b.Run("ParsePacket", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = p.parse(bs, EmptySkipper)
		}
		p.Reset()
	})
	p.Close()
}
