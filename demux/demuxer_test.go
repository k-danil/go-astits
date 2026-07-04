package demux

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v2/internal/bitstest"
	"github.com/k-danil/go-astits/v2/pes"
	"github.com/k-danil/go-astits/v2/psi"
	"github.com/k-danil/go-astits/v2/ts"
)

func hexToBytes(in string) []byte {
	cin := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, in)
	o, err := hex.DecodeString(cin)
	if err != nil {
		panic(err)
	}
	return o
}

func TestDemuxerNew(t *testing.T) {
	ps := 1
	sp := func(p *ts.Packet) bool { return true }
	dmx := New(context.Background(), nil, WithPacketSize(ps), WithPacketSkipper(sp), WithDVBTables())
	assert.Equal(t, uint(ps), dmx.optPacketSize)
	assert.Equal(t, fmt.Sprintf("%p", sp), fmt.Sprintf("%p", dmx.optPacketSkipper))
	assert.True(t, dmx.optDVBTables)
	assert.True(t, dmx.acc.dvbTables)
}

func TestDemuxerNextPacket(t *testing.T) {
	// Ctx error
	ctx, cancel := context.WithCancel(context.Background())
	dmx := New(ctx, bytes.NewReader([]byte{}))
	cancel()
	_, err := dmx.NextPacket()
	assert.Error(t, err)

	// Valid
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	b1, p1 := packet(packetHeader, packetAdaptationField, []byte("1"), true)
	_ = w.Write(b1)
	b2, p2 := packet(packetHeader, packetAdaptationField, []byte("2"), true)
	p2.Offset = int64(len(b1))
	_ = w.Write(b2)
	dmx = New(context.Background(), bytes.NewReader(buf.Bytes()))

	// First packet
	p, err := dmx.NextPacket()
	assert.NoError(t, err)
	assert.Equal(t, b1, p.Raw())
	assert.Equal(t, p1.Header, p.Header)
	assert.Equal(t, p1.AdaptationField, p.AdaptationField)
	assert.Equal(t, p1.Payload, p.Payload)
	assert.Equal(t, p1.Offset, p.Offset)
	assert.Equal(t, uint(192), dmx.packetBuffer.PacketSize())

	// Second packet
	p, err = dmx.NextPacket()
	assert.NoError(t, err)
	assert.Equal(t, b2, p.Raw())
	assert.Equal(t, p2.Header, p.Header)
	assert.Equal(t, p2.AdaptationField, p.AdaptationField)
	assert.Equal(t, p2.Payload, p.Payload)
	assert.Equal(t, p2.Offset, p.Offset)

	// EOF
	_, err = dmx.NextPacket()
	assert.EqualError(t, err, ts.ErrNoMorePackets.Error())
}

func TestDemuxerNextTables(t *testing.T) {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	b := psiBytes()
	b1, _ := packet(ts.PacketHeader{ContinuityCounter: uint8(0), PayloadUnitStartIndicator: true, PID: ts.PIDPAT}, &ts.PacketAdaptationField{}, b[:147], true)
	_ = w.Write(b1)
	b2, _ := packet(ts.PacketHeader{ContinuityCounter: uint8(1), PID: ts.PIDPAT}, &ts.PacketAdaptationField{}, b[147:], true)
	_ = w.Write(b2)
	dmx := New(context.Background(), bytes.NewReader(buf.Bytes()))

	psiData, err := psi.Parse(b)
	require.NoError(t, err)

	var want []psi.SectionSyntaxData
	for _, s := range psiData.Sections {
		if s.Syntax != nil && s.Syntax.Data != nil {
			want = append(want, s.Syntax.Data)
		}
	}
	require.NotEmpty(t, want)

	var got []psi.SectionSyntaxData
	for {
		ev, nerr := dmx.Next()
		if nerr != nil {
			assert.EqualError(t, nerr, ts.ErrNoMorePackets.Error())
			break
		}
		require.NotEqual(t, EventPES, ev)
		pid, data := dmx.Section()
		assert.Equal(t, ts.PIDPAT, pid)
		got = append(got, data)
	}
	assert.Equal(t, want, got)

	// Table state and program map
	assert.NotNil(t, dmx.PAT())
	assert.NotNil(t, dmx.PMT())
	assert.Equal(t, []uint16{0x3, 0x5}, dmx.programMap.Keys)
	assert.Equal(t, []uint16{0x2, 0x4}, dmx.programMap.Vals)
}

func TestDemuxerNextUnknownDataPackets(t *testing.T) {
	buf := &bytes.Buffer{}
	bufWriter := bitstest.NewWriter(buf)

	// ts.Packet that isn't a data packet (PSI or PES)
	b1, _ := packet(ts.PacketHeader{
		ContinuityCounter:         uint8(0),
		PID:                       256,
		PayloadUnitStartIndicator: true,
		HasPayload:                true,
	}, &ts.PacketAdaptationField{}, []byte{0x01, 0x02, 0x03, 0x04}, true)
	bufWriter.Write(b1)

	dmx := New(context.Background(), bytes.NewReader(buf.Bytes()),
		WithPacketSize(188))
	_, err := dmx.Next()
	assert.EqualError(t, err, ts.ErrNoMorePackets.Error())
}

func TestDemuxerNextPATPMT(t *testing.T) {
	pat := hexToBytes(`474000100000b00d0001c100000001f0002ab104b2ffffffffffffffff
		ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff
		ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff
		ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff
		ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff
		ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff
		ffffffffffffffffff`)
	pmt := hexToBytes(`475000100002b0170001c10000e100f0001be100f0000fe101f0002f44
		b99bffffffffffffffffffffffffffffffffffffffffffffffffffffffff
		ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff
		ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff
		ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff
		ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff
		ffffffffffffffffff`)
	r := bytes.NewReader(append(pat, pmt...))
	dmx := New(context.Background(), r, WithPacketSize(188))
	assert.Equal(t, 188*2, r.Len())

	ev, err := dmx.Next()
	assert.NoError(t, err)
	assert.Equal(t, EventPAT, ev)
	pid, data := dmx.Section()
	assert.Equal(t, uint16(0), pid)
	assert.IsType(t, (*psi.PAT)(nil), data)
	assert.NotNil(t, dmx.PAT())
	assert.Equal(t, 188, r.Len())

	ev, err = dmx.Next()
	assert.NoError(t, err)
	assert.Equal(t, EventPMT, ev)
	pid, data = dmx.Section()
	assert.Equal(t, uint16(0x1000), pid)
	assert.IsType(t, (*psi.PMT)(nil), data)
	assert.NotNil(t, dmx.PMT())
}

func TestDemuxerNextPES(t *testing.T) {
	p := pesWithHeaderBytes()
	buf := &bytes.Buffer{}
	writePkt := func(cc uint8, pusi bool, af *ts.PacketAdaptationField, payload []byte) {
		if af == nil {
			af = &ts.PacketAdaptationField{}
		}
		// AF stuffing makes the payload land byte-exact: 184 = 2 AF service
		// bytes + PCR + stuffing + payload
		content := 0
		if af.HasPCR {
			content += ts.PCRSize
		}
		af.StuffingLength = uint8(ts.PacketSize - ts.HeaderSize - 2 - content - len(payload))
		pk := ts.Packet{
			Header: ts.PacketHeader{
				ContinuityCounter:         cc,
				PayloadUnitStartIndicator: pusi,
				PID:                       256,
				HasPayload:                true,
				HasAdaptationField:        true,
			},
			Payload: payload,
		}
		pk.SetAdaptationField(af)
		var bs [ts.PacketSize]byte
		_, err := pk.Put(bs[:])
		require.NoError(t, err)
		buf.Write(bs[:])
	}
	writePkt(0, true, &ts.PacketAdaptationField{RandomAccessIndicator: true, HasPCR: true, PCR: packetAdaptationField.PCR}, p[:33])
	writePkt(1, false, nil, p[33:])
	// Second unit start flushes the first; it drains at EOF itself
	writePkt(2, true, nil, p)

	dmx := New(context.Background(), bytes.NewReader(buf.Bytes()), WithPacketSize(ts.PacketSize))

	ev, err := dmx.Next()
	require.NoError(t, err)
	require.Equal(t, EventPES, ev)
	first := dmx.PES()
	require.NotNil(t, first)
	assert.Equal(t, uint16(256), first.PID)
	assert.Equal(t, uint8(0), first.ContinuityCounter)
	require.NotNil(t, first.AdaptationField)
	assert.True(t, first.AdaptationField.RandomAccessIndicator)
	assert.Equal(t, packetAdaptationField.PCR.Base(), first.AdaptationField.PCR.Base())

	var wantData pes.Data
	require.NoError(t, wantData.Parse(p))
	assert.Equal(t, wantData.Data, first.Data.Data)

	// The claimed unit survives subsequent demuxing
	ev, err = dmx.Next()
	require.NoError(t, err)
	require.Equal(t, EventPES, ev)
	assert.Equal(t, wantData.Data, first.Data.Data)
	// The tail unit is left unclaimed on purpose: the next call releases it

	_, err = dmx.Next()
	assert.EqualError(t, err, ts.ErrNoMorePackets.Error())
	first.Close()
	first.Close() // idempotent
	dmx.Close()
}

func TestDemuxerRewind(t *testing.T) {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	b := psiBytes()
	b1, _ := packet(ts.PacketHeader{ContinuityCounter: uint8(0), PayloadUnitStartIndicator: true, PID: ts.PIDPAT}, &ts.PacketAdaptationField{}, b[:147], true)
	_ = w.Write(b1)
	b2, _ := packet(ts.PacketHeader{ContinuityCounter: uint8(1), PID: ts.PIDPAT}, &ts.PacketAdaptationField{}, b[147:], true)
	_ = w.Write(b2)
	r := bytes.NewReader(buf.Bytes())
	dmx := New(context.Background(), r)

	countEvents := func() (n int) {
		for {
			_, err := dmx.Next()
			if err != nil {
				require.True(t, errors.Is(err, ts.ErrNoMorePackets))
				return
			}
			n++
		}
	}

	first := countEvents()
	require.NotZero(t, first)

	n, err := dmx.Rewind()
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)
	assert.Nil(t, dmx.packetBuffer)
	assert.Equal(t, buf.Len(), r.Len())

	// The dedup cache is gone: everything re-emits; the program map survives
	assert.Equal(t, first, countEvents())
	assert.Equal(t, []uint16{0x3, 0x5}, dmx.programMap.Keys)
}

func BenchmarkDemuxer_Next(b *testing.B) {
	b.ReportAllocs()

	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	bs := psiBytes()
	b1, _ := packet(ts.PacketHeader{ContinuityCounter: uint8(0), PayloadUnitStartIndicator: true, PID: ts.PIDPAT}, &ts.PacketAdaptationField{}, bs[:147], true)
	_ = w.Write(b1)
	b2, _ := packet(ts.PacketHeader{ContinuityCounter: uint8(1), PID: ts.PIDPAT}, &ts.PacketAdaptationField{}, bs[147:], true)
	_ = w.Write(b2)

	r := bytes.NewReader(buf.Bytes())
	dmx := New(context.Background(), r)

	for i := 0; i < b.N; i++ {
		_, _ = dmx.Rewind()
		for {
			if _, err := dmx.Next(); err != nil {
				break
			}
		}
	}
}

func fuzzSeedStream() []byte {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	bs := psiBytes()
	b1, _ := packet(ts.PacketHeader{ContinuityCounter: uint8(0), PayloadUnitStartIndicator: true, PID: ts.PIDPAT}, &ts.PacketAdaptationField{}, bs[:147], true)
	w.Write(b1)
	b2, _ := packet(ts.PacketHeader{ContinuityCounter: uint8(1), PID: ts.PIDPAT}, &ts.PacketAdaptationField{}, bs[147:], true)
	w.Write(b2)
	return buf.Bytes()
}

func FuzzDemuxer(f *testing.F) {
	f.Add(fuzzSeedStream())
	f.Add(bytes.Repeat([]byte{0x47}, 188*3))
	f.Fuzz(func(t *testing.T, b []byte) {
		dmx := New(context.Background(), bytes.NewReader(b), WithPacketSize(188), WithDVBTables())
		for {
			ev, err := dmx.Next()
			if err != nil {
				break
			}
			if ev == EventPES {
				dmx.PES()
			}
		}
		dmx.Close()
	})
}

// FuzzDemuxerView exercises packet size autodetection and Next over the
// zero-copy batch path.
func FuzzDemuxerView(f *testing.F) {
	f.Add(fuzzSeedStream())
	f.Add(bytes.Repeat([]byte{0x47}, 188*3))
	f.Fuzz(func(t *testing.T, b []byte) {
		dmx := New(context.Background(), bytes.NewReader(b), WithZeroCopyPackets(4), WithDVBTables())
		for {
			if _, err := dmx.Next(); err != nil {
				break
			}
		}
		dmx.Close()
	})
}

func TestDemuxerPSIRepeats(t *testing.T) {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	b := psiBytes()
	cc := uint8(0)
	writePSI := func() {
		b1, _ := packet(ts.PacketHeader{ContinuityCounter: cc, PayloadUnitStartIndicator: true, PID: ts.PIDPAT}, &ts.PacketAdaptationField{}, b[:147], true)
		_ = w.Write(b1)
		cc++
		b2, _ := packet(ts.PacketHeader{ContinuityCounter: cc, PID: ts.PIDPAT}, &ts.PacketAdaptationField{}, b[147:], true)
		_ = w.Write(b2)
		cc++
	}
	const copies = 3
	for i := 0; i < copies; i++ {
		writePSI()
	}
	stream := buf.Bytes()

	// Default: identical repeats suppressed — one round of events, all changed
	def := New(context.Background(), bytes.NewReader(stream))
	var defEvents, defChanged int
	for {
		ev, err := def.Next()
		if err != nil {
			require.True(t, errors.Is(err, ts.ErrNoMorePackets))
			break
		}
		require.NotEqual(t, EventPES, ev)
		defEvents++
		if def.TableChanged() {
			defChanged++
		}
	}
	require.NotZero(t, defEvents)
	assert.Equal(t, defEvents, defChanged, "every default event is a content change")

	// WithPSIRepeats: every copy emits; only the first round is changed
	rep := New(context.Background(), bytes.NewReader(stream), WithPSIRepeats())
	var repEvents, repChanged int
	for {
		ev, err := rep.Next()
		if err != nil {
			require.True(t, errors.Is(err, ts.ErrNoMorePackets))
			break
		}
		require.NotEqual(t, EventPES, ev)
		repEvents++
		if rep.TableChanged() {
			repChanged++
		}
	}
	assert.Equal(t, defEvents*copies, repEvents, "each of %d copies re-emits", copies)
	assert.Equal(t, defEvents, repChanged, "only the first copy is a content change")
}
