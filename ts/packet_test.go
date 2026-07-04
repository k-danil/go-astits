package ts

import (
	"bytes"
	"fmt"
	"github.com/asticode/go-astikit"
	"github.com/stretchr/testify/assert"
	"testing"
)

func packet(h PacketHeader, a *PacketAdaptationField, i []byte, packet192bytes bool) ([]byte, *Packet) {
	buf := &bytes.Buffer{}
	w := astikit.NewBitsWriter(astikit.BitsWriterOptions{Writer: buf})
	w.Write(syncByte) // Sync byte
	if packet192bytes {
		w.Write([]byte("test")) // Sometimes packets are 192 bytes
	}
	w.Write(packetHeaderBytes(h, "11"))                             // Header
	w.Write(packetAdaptationFieldBytes(a))                          // Adaptation field
	var payload = append(i, bytes.Repeat([]byte{0}, 147-len(i))...) // Payload
	w.Write(payload)
	pk := &Packet{
		Header:  packetHeader,
		Payload: payload,
	}
	pk.af = *packetAdaptationField
	pk.AdaptationField = &pk.af
	return buf.Bytes(), pk
}

func packetShort(h PacketHeader, payload []byte) ([]byte, *Packet) {
	buf := &bytes.Buffer{}
	w := astikit.NewBitsWriter(astikit.BitsWriterOptions{Writer: buf})
	w.Write(syncByte)                   // Sync byte
	w.Write(packetHeaderBytes(h, "01")) // Header
	p := append(payload, bytes.Repeat([]byte{0}, MpegTsPacketSize-buf.Len())...)
	w.Write(p)
	return buf.Bytes(), &Packet{
		Header:  h,
		Payload: payload,
	}
}

func TestParsePacket(t *testing.T) {
	// Packet not starting with a sync
	bs := make([]byte, MpegTsPacketSize)
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
	buf := &bytes.Buffer{}
	scratch := make([]byte, MpegTsPacketSize)
	n, err := ep.WriteBytes(scratch, buf)
	assert.NoError(t, err)
	assert.Equal(t, MpegTsPacketSize, n)
	assert.Equal(t, n, buf.Len())
	assert.Equal(t, len(eb), buf.Len())
	assert.Equal(t, eb, buf.Bytes())
}

func BenchmarkWritePacket(b *testing.B) {
	_, ep := packet(packetHeader, packetAdaptationField, []byte("payload"), false)
	buf := &bytes.Buffer{}
	buf.Grow(200)
	scratch := make([]byte, MpegTsPacketSize)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		ep.WriteBytes(scratch, buf)
	}
}

func TestWritePacket_HeaderOnly(t *testing.T) {
	shortPacketHeader := packetHeader
	shortPacketHeader.HasPayload = false
	shortPacketHeader.HasAdaptationField = false
	_, ep := packetShort(shortPacketHeader, nil)

	buf := &bytes.Buffer{}
	n, err := ep.WriteBytes(make([]byte, MpegTsPacketSize), buf)
	assert.NoError(t, err)
	assert.Equal(t, MpegTsPacketSize, n)
	assert.Equal(t, n, buf.Len())

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
	w := astikit.NewBitsWriter(astikit.BitsWriterOptions{Writer: buf})
	w.Write(h.TransportErrorIndicator)                // Transport error indicator
	w.Write(h.PayloadUnitStartIndicator)              // Payload unit start indicator
	w.Write("1")                                      // Transport priority
	w.Write(fmt.Sprintf("%.13b", h.PID))              // PID
	w.Write("10")                                     // Scrambling control
	w.Write(afControl)                                // Adaptation field control
	w.Write(fmt.Sprintf("%.4b", h.ContinuityCounter)) // Continuity counter
	return buf.Bytes()
}

func TestParsePacketHeader(t *testing.T) {
	v := PacketHeader{}
	v.parseBytes(packetHeaderBytes(packetHeader, "11"))
	assert.Equal(t, packetHeader, v)
}

func TestWritePacketHeader(t *testing.T) {
	buf := &bytes.Buffer{}
	w := astikit.NewBitsWriter(astikit.BitsWriterOptions{Writer: buf})
	bb := new([8]byte)
	header := append([]byte{syncByte}, packetHeaderBytes(packetHeader, "11")...)
	bytesWritten, err := packetHeader.write(w, bb[:])
	assert.NoError(t, err)
	assert.Equal(t, bytesWritten, 4)
	assert.Equal(t, bytesWritten, buf.Len())
	assert.Equal(t, header, buf.Bytes())
}

func BenchmarkWritePacketHeader(b *testing.B) {
	buf := &bytes.Buffer{}
	buf.Grow(3)
	w := astikit.NewBitsWriter(astikit.BitsWriterOptions{Writer: buf})

	bb := new([8]byte)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		packetHeader.write(w, bb[:])
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
	privBuf:                           [transportPrivateDataMaxSize]byte{'t', 'e', 's', 't'},
	StuffingLength:                    5,
}

func packetAdaptationFieldBytes(a *PacketAdaptationField) []byte {
	buf := &bytes.Buffer{}
	w := astikit.NewBitsWriter(astikit.BitsWriterOptions{Writer: buf})
	w.Write(uint8(36))                // Length
	w.Write(a.DiscontinuityIndicator) // Discontinuity indicator
	w.Write("1")                      // Random access indicator
	w.Write("1")                      // Elementary stream priority indicator
	w.Write("1")                      // PCR flag
	w.Write("1")                      // OPCR flag
	w.Write("1")                      // Splicing point flag
	w.Write("1")                      // Transport data flag
	w.Write("1")                      // Adaptation field extension flag
	w.Write(pcrBytes())               // PCR
	w.Write(pcrBytes())               // OPCR
	w.Write(uint8(2))                 // Splice countdown
	w.Write(uint8(4))                 // Transport private data length
	w.Write([]byte("test"))           // Transport private data
	w.Write(uint8(11))                // Adaptation extension length
	w.Write("1")                      // LTW flag
	w.Write("1")                      // Piecewise rate flag
	w.Write("1")                      // Seamless splice flag
	w.Write("11111")                  // Reserved
	w.Write("1")                      // LTW valid flag
	w.Write("010101010101010")        // LTW offset
	w.Write("11")                     // Piecewise rate reserved
	w.Write("1010101010101010101010") // Piecewise rate
	w.Write(dtsBytes("0010"))         // Splice type + DTS next access unit
	w.WriteN(^uint64(0), 40)          // Stuffing bytes
	return buf.Bytes()
}

func TestParsePacketAdaptationField(t *testing.T) {
	af := &PacketAdaptationField{}
	err := af.parseBytes(packetAdaptationFieldBytes(packetAdaptationField), 0)
	assert.Equal(t, packetAdaptationField, af)
	assert.NoError(t, err)
}

func TestWritePacketAdaptationField(t *testing.T) {
	eb := packetAdaptationFieldBytes(packetAdaptationField)
	bs := make([]byte, MpegTsPacketSize)
	bytesWritten, err := packetAdaptationField.putBytes(bs)
	assert.NoError(t, err)
	assert.Equal(t, len(eb), bytesWritten)
	assert.Equal(t, eb, bs[:bytesWritten])
}

func BenchmarkWritePacketAdaptationField(b *testing.B) {
	bs := make([]byte, MpegTsPacketSize)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		packetAdaptationField.putBytes(bs)
	}
}

var pcr = NewClockReference(5726623061, 341)

func pcrBytes() []byte {
	buf := &bytes.Buffer{}
	w := astikit.NewBitsWriter(astikit.BitsWriterOptions{Writer: buf})
	w.Write("101010101010101010101010101010101") // Base
	w.Write("111111")                            // Reserved
	w.Write("101010101")                         // Extension
	return buf.Bytes()
}

func TestParsePCR(t *testing.T) {
	v := parsePCRBytes(pcrBytes())
	assert.Equal(t, pcr, v)
}

func BenchmarkParsePCR(b *testing.B) {
	b.ReportAllocs()

	bs := pcrBytes()

	var v ClockReference
	for i := 0; i < b.N; i++ {
		v = parsePCRBytes(bs)
	}
	_ = v
}

func TestWritePCR(t *testing.T) {
	bs := make([]byte, pcrBytesSize)
	bytesWritten := pcr.putPCRBytes(bs)
	assert.Equal(t, bytesWritten, 6)
	assert.Equal(t, pcrBytes(), bs)
}

func BenchmarkWritePCR(b *testing.B) {
	bs := make([]byte, pcrBytesSize)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		pcr.putPCRBytes(bs)
	}
}

func BenchmarkParsePacket(b *testing.B) {
	bs, _ := packet(packetHeader, packetAdaptationField, []byte("payload"), true)

	b.Run("ParsePacket", func(b *testing.B) {
		b.ReportAllocs()

		p := NewPacket()
		for i := 0; i < b.N; i++ {
			p.parse(bs, EmptySkipper)
		}
		p.Close()
	})
}
