package mux

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v3/demux"
	"github.com/k-danil/go-astits/v3/internal/bitstest"
	"github.com/k-danil/go-astits/v3/pes"
	"github.com/k-danil/go-astits/v3/psi"
	"github.com/k-danil/go-astits/v3/ts"
)

const syncByte byte = '\x47'

func patExpectedBytes(versionNumber uint8, cc uint8) []byte {
	buf := bytes.Buffer{}
	w := bitstest.NewWriter(&buf)
	_ = w.Write(syncByte)
	_ = w.Write("010") // no transport error, payload start, no priority
	_ = w.WriteN(ts.PIDPAT, 13)
	_ = w.Write("0001") // no scrambling, no AF, payload present
	_ = w.WriteN(cc, 4)

	_ = w.Write(uint16(0))       // Table ID
	_ = w.Write("1011")          // Syntax section indicator, private bit, reserved
	_ = w.WriteN(uint16(13), 12) // Section length

	_ = w.Write(uint16(psi.TableIDPAT))
	_ = w.Write("11")              // Reserved bits
	_ = w.WriteN(versionNumber, 5) // Version number
	_ = w.Write("1")               // Current/next indicator
	_ = w.Write(uint8(0))          // Section number
	_ = w.Write(uint8(0))          // Last section number

	_ = w.Write(programNumberStart)
	_ = w.Write("111") // reserved
	_ = w.WriteN(pmtStartPID, 13)

	// CRC32
	if versionNumber == 0 {
		_ = w.Write([]byte{0x71, 0x10, 0xd8, 0x78})
	} else {
		_ = w.Write([]byte{0xef, 0xbe, 0x08, 0x5a})
	}

	_ = w.Write(bytes.Repeat([]byte{0xff}, 167))

	return buf.Bytes()
}

func TestMuxer_generatePAT(t *testing.T) {
	muxer := New(context.Background(), nil)

	err := muxer.generatePAT()
	require.NoError(t, err)
	patchContinuityCounters(muxer.patBytes.Bytes(), &muxer.patCC)
	assert.Equal(t, ts.PacketSize, muxer.patBytes.Len())
	assert.Equal(t, patExpectedBytes(0, 0), muxer.patBytes.Bytes())

	// Version number shouldn't change
	err = muxer.generatePAT()
	require.NoError(t, err)
	patchContinuityCounters(muxer.patBytes.Bytes(), &muxer.patCC)
	assert.Equal(t, ts.PacketSize, muxer.patBytes.Len())
	assert.Equal(t, patExpectedBytes(0, 1), muxer.patBytes.Bytes())

	// Version number should change
	muxer.pmUpdated = true
	err = muxer.generatePAT()
	require.NoError(t, err)
	patchContinuityCounters(muxer.patBytes.Bytes(), &muxer.patCC)
	assert.Equal(t, ts.PacketSize, muxer.patBytes.Len())
	assert.Equal(t, patExpectedBytes(1, 2), muxer.patBytes.Bytes())
}

func pmtExpectedBytesVideoOnly(versionNumber, cc uint8) []byte {
	buf := bytes.Buffer{}
	w := bitstest.NewWriter(&buf)
	_ = w.Write(syncByte)
	_ = w.Write("010") // no transport error, payload start, no priority
	_ = w.WriteN(pmtStartPID, 13)
	_ = w.Write("0001") // no scrambling, no AF, payload present
	_ = w.WriteN(cc, 4)

	_ = w.Write(uint16(psi.TableIDPMT)) // Table ID
	_ = w.Write("1011")                 // Syntax section indicator, private bit, reserved
	_ = w.WriteN(uint16(18), 12)        // Section length

	_ = w.Write(programNumberStart)
	_ = w.Write("11")              // Reserved bits
	_ = w.WriteN(versionNumber, 5) // Version number
	_ = w.Write("1")               // Current/next indicator
	_ = w.Write(uint8(0))          // Section number
	_ = w.Write(uint8(0))          // Last section number

	_ = w.Write("111")               // reserved
	_ = w.WriteN(uint16(0x1234), 13) // PCR PID

	_ = w.Write("1111")         // reserved
	_ = w.WriteN(uint16(0), 12) // program info length

	_ = w.Write(uint8(psi.StreamTypeH264Video))
	_ = w.Write("111") // reserved
	_ = w.WriteN(uint16(0x1234), 13)

	_ = w.Write("1111")         // reserved
	_ = w.WriteN(uint16(0), 12) // es info length

	_ = w.Write([]byte{0x31, 0x48, 0x5b, 0xa2}) // CRC32

	_ = w.Write(bytes.Repeat([]byte{0xff}, 162))

	return buf.Bytes()
}

func pmtExpectedBytesVideoAndAudio(versionNumber uint8, cc uint8) []byte {
	buf := bytes.Buffer{}
	w := bitstest.NewWriter(&buf)
	_ = w.Write(syncByte)
	_ = w.Write("010") // no transport error, payload start, no priority
	_ = w.WriteN(pmtStartPID, 13)
	_ = w.Write("0001") // no scrambling, no AF, payload present
	_ = w.WriteN(cc, 4)

	_ = w.Write(uint16(psi.TableIDPMT)) // Table ID
	_ = w.Write("1011")                 // Syntax section indicator, private bit, reserved
	_ = w.WriteN(uint16(23), 12)        // Section length

	_ = w.Write(programNumberStart)
	_ = w.Write("11")              // Reserved bits
	_ = w.WriteN(versionNumber, 5) // Version number
	_ = w.Write("1")               // Current/next indicator
	_ = w.Write(uint8(0))          // Section number
	_ = w.Write(uint8(0))          // Last section number

	_ = w.Write("111")               // reserved
	_ = w.WriteN(uint16(0x1234), 13) // PCR PID

	_ = w.Write("1111")         // reserved
	_ = w.WriteN(uint16(0), 12) // program info length

	_ = w.Write(uint8(psi.StreamTypeH264Video))
	_ = w.Write("111") // reserved
	_ = w.WriteN(uint16(0x1234), 13)
	_ = w.Write("1111")         // reserved
	_ = w.WriteN(uint16(0), 12) // es info length

	_ = w.Write(uint8(psi.StreamTypeADTS))
	_ = w.Write("111") // reserved
	_ = w.WriteN(uint16(0x0234), 13)
	_ = w.Write("1111")         // reserved
	_ = w.WriteN(uint16(0), 12) // es info length

	// CRC32
	if versionNumber == 0 {
		_ = w.Write([]byte{0x29, 0x52, 0xc4, 0x50})
	} else {
		_ = w.Write([]byte{0x06, 0xf4, 0xa6, 0xea})
	}

	_ = w.Write(bytes.Repeat([]byte{0xff}, 157))

	return buf.Bytes()
}

func TestMuxer_generatePMT(t *testing.T) {
	muxer := New(context.Background(), nil)
	err := muxer.AddElementaryStream(psi.ElementaryStream{
		ElementaryPID: 0x1234,
		StreamType:    psi.StreamTypeH264Video,
	})
	muxer.SetPCRPID(0x1234)
	require.NoError(t, err)

	err = muxer.generatePMT()
	require.NoError(t, err)
	patchContinuityCounters(muxer.pmtBytes.Bytes(), &muxer.pmtCC)
	assert.Equal(t, ts.PacketSize, muxer.pmtBytes.Len())
	assert.Equal(t, pmtExpectedBytesVideoOnly(0, 0), muxer.pmtBytes.Bytes())

	// Version number shouldn't change
	err = muxer.generatePMT()
	require.NoError(t, err)
	patchContinuityCounters(muxer.pmtBytes.Bytes(), &muxer.pmtCC)
	assert.Equal(t, ts.PacketSize, muxer.pmtBytes.Len())
	assert.Equal(t, pmtExpectedBytesVideoOnly(0, 1), muxer.pmtBytes.Bytes())

	err = muxer.AddElementaryStream(psi.ElementaryStream{
		ElementaryPID: 0x0234,
		StreamType:    psi.StreamTypeAACAudio,
	})
	require.NoError(t, err)

	// Version number should change
	err = muxer.generatePMT()
	require.NoError(t, err)
	patchContinuityCounters(muxer.pmtBytes.Bytes(), &muxer.pmtCC)
	assert.Equal(t, ts.PacketSize, muxer.pmtBytes.Len())
	assert.Equal(t, pmtExpectedBytesVideoAndAudio(1, 2), muxer.pmtBytes.Bytes())
}

func TestMuxer_WriteDataMultiPacket(t *testing.T) {
	// A payload spanning many packets exercises the fast mid-unit path (fixed
	// header + full payload chunk, no PES header, no adaptation field). Demuxing
	// the output back must reproduce the payload byte for byte.
	buf := &bytes.Buffer{}
	m := New(context.Background(), buf)
	const pid = 0x100
	require.NoError(t, m.AddElementaryStream(psi.ElementaryStream{ElementaryPID: pid, StreamType: psi.StreamTypeH264Video}))
	m.SetPCRPID(pid)

	payload := make([]byte, 2000) // ~11 packets → several full mid-unit ones
	for i := range payload {
		payload[i] = byte(i * 7)
	}
	pts := ts.NewClockReference(90000, 0)
	writeUnit := func() {
		_, err := m.WriteData(&Data{
			PID:             pid,
			AdaptationField: &ts.PacketAdaptationField{HasPCR: true, PCR: pts, RandomAccessIndicator: true},
			PES: &pes.Data{
				Data:   payload,
				Header: pes.Header{OptionalHeader: &pes.OptionalHeader{PTS: pts, PTSDTSIndicator: pes.PTSDTSIndicatorOnlyPTS}},
			},
		})
		require.NoError(t, err)
	}
	writeUnit()
	writeUnit() // the second unit's PUSI flushes the first on demux

	dmx := demux.New(context.Background(), bytes.NewReader(buf.Bytes()), demux.WithPacketSize(ts.PacketSize))
	for {
		ev, derr := dmx.Next()
		require.NoError(t, derr, "PES unit not emitted before EOF")
		if ev != demux.EventPES {
			continue
		}
		got := dmx.PES()
		assert.Equal(t, uint16(pid), got.PID)
		assert.Equal(t, payload, got.Data.Data)
		return
	}
}

func TestMuxer_WriteDataFatAdaptationField(t *testing.T) {
	// A large adaptation field (transport private data) leaves no room for the
	// PES header in the first packet, so the header spans into the next. Demuxing
	// must recover the adaptation field (PCR/RAI/private data) and the payload.
	buf := &bytes.Buffer{}
	m := New(context.Background(), buf)
	const pid = 0x100
	require.NoError(t, m.AddElementaryStream(psi.ElementaryStream{ElementaryPID: pid, StreamType: psi.StreamTypeH264Video}))
	m.SetPCRPID(pid)

	priv := make([]byte, 170)
	for i := range priv {
		priv[i] = byte(i)
	}
	payload := make([]byte, 500)
	for i := range payload {
		payload[i] = byte(i * 3)
	}
	pts := ts.NewClockReference(90000, 0)
	writeUnit := func() {
		_, err := m.WriteData(&Data{
			PID: pid,
			AdaptationField: &ts.PacketAdaptationField{
				HasPCR: true, PCR: pts, RandomAccessIndicator: true,
				HasTransportPrivateData: true, TransportPrivateData: priv, TransportPrivateDataLength: uint8(len(priv)),
			},
			PES: &pes.Data{Data: payload, Header: pes.Header{OptionalHeader: &pes.OptionalHeader{PTS: pts, PTSDTSIndicator: pes.PTSDTSIndicatorOnlyPTS}}},
		})
		require.NoError(t, err)
	}
	writeUnit()
	writeUnit() // the second unit's PUSI flushes the first on demux

	dmx := demux.New(context.Background(), bytes.NewReader(buf.Bytes()), demux.WithPacketSize(ts.PacketSize))
	for {
		ev, derr := dmx.Next()
		require.NoError(t, derr, "PES unit not emitted before EOF")
		if ev != demux.EventPES {
			continue
		}
		got := dmx.PES()
		require.NotNil(t, got.AdaptationField, "adaptation field survived the span")
		assert.True(t, got.AdaptationField.RandomAccessIndicator)
		assert.Equal(t, pts, got.AdaptationField.PCR)
		assert.Equal(t, priv, got.AdaptationField.TransportPrivateData)
		assert.Equal(t, payload, got.Data.Data)
		return
	}
}

func packetsOn(t *testing.T, stream []byte, pid uint16) (out [][]byte) {
	t.Helper()
	require.Zero(t, len(stream)%ts.PacketSize)
	for off := 0; off+ts.PacketSize <= len(stream); off += ts.PacketSize {
		pkt := stream[off : off+ts.PacketSize]
		var h ts.PacketHeader
		_, err := h.Parse(pkt)
		require.NoError(t, err)
		if h.PID == pid {
			out = append(out, pkt)
		}
	}
	return
}

func writeVideoUnit(t *testing.T, m *Muxer, pid uint16, af *ts.PacketAdaptationField, data *pes.Data) {
	t.Helper()
	_, err := m.WriteData(&Data{PID: pid, AdaptationField: af, PES: data})
	require.NoError(t, err)
}

func newVideoMuxer(t *testing.T, w io.Writer, pid uint16) (m *Muxer) {
	t.Helper()
	m = New(context.Background(), w)
	require.NoError(t, m.AddElementaryStream(psi.ElementaryStream{ElementaryPID: pid, StreamType: psi.StreamTypeH264Video}))
	m.SetPCRPID(pid)
	return
}

// A remuxed adaptation field carries its sender's stuffing: without re-measuring
// it the packet declares one length and the PES header starts at another.
func TestMuxer_WriteDataRemuxedAdaptationField(t *testing.T) {
	const pid = 0x100
	payload := make([]byte, 100)
	for i := range payload {
		payload[i] = byte(i)
	}
	pcr := ts.NewClockReference(90000, 0)
	unit := func() *pes.Data {
		return &pes.Data{
			Data:   payload,
			Header: pes.Header{OptionalHeader: &pes.OptionalHeader{PTS: pcr, PTSDTSIndicator: pes.PTSDTSIndicatorOnlyPTS}},
		}
	}

	src := &bytes.Buffer{}
	m := newVideoMuxer(t, src, pid)
	writeVideoUnit(t, m, pid, &ts.PacketAdaptationField{HasPCR: true, PCR: pcr, RandomAccessIndicator: true}, unit())
	writeVideoUnit(t, m, pid, &ts.PacketAdaptationField{HasPCR: true, PCR: pcr, RandomAccessIndicator: true}, unit())

	dmx := demux.New(context.Background(), bytes.NewReader(src.Bytes()), demux.WithPacketSize(ts.PacketSize))
	var got *demux.PES
	for got == nil {
		ev, derr := dmx.Next()
		require.NoError(t, derr, "PES unit not emitted before EOF")
		if ev == demux.EventPES {
			got = dmx.PES()
		}
	}
	require.NotNil(t, got.AdaptationField)
	require.NotZero(t, got.AdaptationField.StuffingLength, "the parsed field reports the sender's stuffing")

	out := &bytes.Buffer{}
	m2 := newVideoMuxer(t, out, pid)
	writeVideoUnit(t, m2, pid, got.AdaptationField, &got.Data)

	first := packetsOn(t, out.Bytes(), pid)[0]
	afLen := int(first[ts.HeaderSize])
	assert.Equal(t, []byte{0x00, 0x00, 0x01}, first[ts.HeaderSize+1+afLen:ts.HeaderSize+4+afLen],
		"the PES header starts right after the declared adaptation field")
}

// A field that cannot share its packet with any payload is refused instead of
// driving the layout arithmetic negative.
func TestMuxer_WriteDataAdaptationFieldTooLong(t *testing.T) {
	const pid = 0x100
	m := newVideoMuxer(t, &bytes.Buffer{}, pid)
	_, err := m.WriteData(&Data{
		PID: pid,
		AdaptationField: &ts.PacketAdaptationField{
			HasTransportPrivateData: true,
			TransportPrivateData:    make([]byte, 250),
		},
		PES: &pes.Data{Data: []byte{0x01}, Header: pes.Header{
			StreamID:       0xe0,
			OptionalHeader: &pes.OptionalHeader{PTS: ts.NewClockReference(90000, 0), PTSDTSIndicator: pes.PTSDTSIndicatorOnlyPTS},
		}},
	})
	require.ErrorIs(t, err, ErrAdaptationFieldTooLong)
}

type failFirstWriter struct {
	w      io.Writer
	failed bool
}

var errWriteRefused = errors.New("write refused")

func (w *failFirstWriter) Write(p []byte) (n int, err error) {
	if !w.failed {
		w.failed = true
		return 0, errWriteRefused
	}
	return w.w.Write(p)
}

func TestWriteTablesKeepsCCWhenTheWriteFails(t *testing.T) {
	const pid = 0x100
	out := &bytes.Buffer{}
	m := newVideoMuxer(t, &failFirstWriter{w: out}, pid)

	_, err := m.WriteTables()
	require.ErrorIs(t, err, errWriteRefused)
	require.Zero(t, out.Len())

	_, err = m.WriteTables()
	require.NoError(t, err)

	for _, tablePID := range []uint16{ts.PIDPAT, pmtStartPID} {
		pkts := packetsOn(t, out.Bytes(), tablePID)
		require.Len(t, pkts, 1)
		var h ts.PacketHeader
		_, err = h.Parse(pkts[0])
		require.NoError(t, err)
		assert.Zero(t, h.ContinuityCounter, "PID %#x", tablePID)
	}
}

// H.222.0 2.4.3.3: a packet with an adaptation field and no payload repeats the
// continuity counter; advancing it there reads as a gap at the decoder.
func TestMuxer_WriteDataAdaptationFieldOnlyKeepsCC(t *testing.T) {
	const pid = 0x100
	// 1 flags byte + 6 PCR + 1 length byte + private data = 183, the whole packet body.
	priv := make([]byte, packetMaxPayload-9)
	pcr := ts.NewClockReference(90000, 0)

	out := &bytes.Buffer{}
	m := newVideoMuxer(t, out, pid)
	writeVideoUnit(t, m, pid, &ts.PacketAdaptationField{
		HasPCR: true, PCR: pcr,
		HasTransportPrivateData: true, TransportPrivateData: priv,
	}, &pes.Data{
		Data:   make([]byte, 200),
		Header: pes.Header{OptionalHeader: &pes.OptionalHeader{PTS: pcr, PTSDTSIndicator: pes.PTSDTSIndicatorOnlyPTS}},
	})

	pkts := packetsOn(t, out.Bytes(), pid)
	require.GreaterOrEqual(t, len(pkts), 3)
	type state struct {
		hasPayload bool
		cc         uint8
	}
	var got []state
	for _, pkt := range pkts {
		var h ts.PacketHeader
		_, err := h.Parse(pkt)
		require.NoError(t, err)
		got = append(got, state{hasPayload: h.HasPayload, cc: h.ContinuityCounter})
	}
	require.False(t, got[0].hasPayload, "the field fills the first packet on its own")
	assert.Equal(t, []state{{false, 0}, {true, 0}, {true, 1}}, got[:3])
}

func BenchmarkMuxWriteDataToBuffer(b *testing.B) {
	payload := make([]byte, 64<<10) // ~350 packets, mostly full mid-unit
	for i := range payload {
		payload[i] = byte(i)
	}
	pts := ts.NewClockReference(90000, 0)
	out := &bytes.Buffer{}
	m := New(context.Background(), out)
	_ = m.AddElementaryStream(psi.ElementaryStream{ElementaryPID: 0x100, StreamType: psi.StreamTypeH264Video})
	m.SetPCRPID(0x100)
	d := &Data{
		PID:             0x100,
		AdaptationField: &ts.PacketAdaptationField{HasPCR: true, PCR: pts, RandomAccessIndicator: true},
		PES:             &pes.Data{Data: payload, Header: pes.Header{OptionalHeader: &pes.OptionalHeader{PTS: pts, PTSDTSIndicator: pes.PTSDTSIndicatorOnlyPTS}}},
	}
	b.SetBytes(int64(len(payload)))
	for b.Loop() {
		out.Reset()
		if _, err := m.WriteData(d); err != nil {
			b.Fatal(err)
		}
	}
}

func testPayload() []byte {
	ret := make([]byte, 0x100)
	for i := range ret {
		ret[i] = byte(i)
	}
	return ret
}

func TestMuxer_WritePayload(t *testing.T) {
	buf := bytes.Buffer{}
	muxer := New(context.Background(), &buf)
	require.NoError(t, muxer.AddElementaryStream(psi.ElementaryStream{
		ElementaryPID: 0x1234,
		StreamType:    psi.StreamTypeH264Video,
	}))
	muxer.SetPCRPID(0x1234)

	pcr := ts.NewClockReference(5726623061, 85)
	pts := ts.NewClockReference(5726623060, 0)
	n, err := muxer.WriteData(&Data{
		PID:             0x1234,
		AdaptationField: &ts.PacketAdaptationField{HasPCR: true, PCR: pcr, RandomAccessIndicator: true},
		PES: &pes.Data{
			Data:   testPayload(),
			Header: pes.Header{OptionalHeader: &pes.OptionalHeader{DTS: pts, PTS: pts, PTSDTSIndicator: pes.PTSDTSIndicatorBothPresent}},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, buf.Len(), n, "WriteData must account for every byte it wrote")
	assert.Equal(t, 0, buf.Len()%ts.PacketSize)
}

// A PAT with more programs than fit one section must span sections and
// packets, and survive a full mux->demux cycle.
func TestWriteTablesMultiSectionPAT(t *testing.T) {
	buf := &bytes.Buffer{}
	m := New(context.Background(), buf)
	require.NoError(t, m.AddElementaryStream(psi.ElementaryStream{
		ElementaryPID: 0x123,
		StreamType:    psi.StreamTypeH264Video,
	}))
	m.SetPCRPID(0x123)

	const extraPrograms = 300
	for i := range extraPrograms {
		m.pm.Set(uint16(0x200+i), uint16(i+1))
	}
	m.pmUpdated = true

	_, err := m.WriteTables()
	require.NoError(t, err)

	dmx := demux.New(context.Background(), bytes.NewReader(buf.Bytes()), demux.WithPacketSize(ts.PacketSize))
	got := map[uint16]uint16{}
	for {
		ev, derr := dmx.Next()
		if errors.Is(derr, ts.ErrNoMorePackets) {
			break
		}
		require.NoError(t, derr)
		if ev != demux.EventPAT {
			continue
		}
		if _, sec := dmx.Section(); sec != nil {
			if pat, isPAT := sec.Syntax.Data.(*psi.PAT); isPAT {
				for _, p := range pat.Programs {
					got[p.ProgramMapID] = p.ProgramNumber
				}
			}
		}
	}
	assert.Len(t, got, extraPrograms+1)
	assert.Equal(t, uint16(42+1), got[uint16(0x200+42)])
}
