package demux

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v3/internal/bitstest"
	"github.com/k-danil/go-astits/v3/pes"
	"github.com/k-danil/go-astits/v3/ts"
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

func TestDemuxerNextPacket(t *testing.T) {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	b1, p1 := packet([]byte("1"), true)
	_ = w.Write(b1)
	b2, p2 := packet([]byte("2"), true)
	p2.Offset = int64(len(b1))
	_ = w.Write(b2)
	dmx := New(context.Background(), bytes.NewReader(buf.Bytes()))

	p, err := dmx.NextPacket()
	assert.NoError(t, err)
	assert.Equal(t, b1, p.Raw())
	assert.Equal(t, p1.Offset, p.Offset)
	assert.Equal(t, uint(192), dmx.packetBuffer.PacketSize())

	p, err = dmx.NextPacket()
	assert.NoError(t, err)
	assert.Equal(t, b2, p.Raw())
	assert.Equal(t, p2.Offset, p.Offset)

	// EOF
	_, err = dmx.NextPacket()
	assert.EqualError(t, err, ts.ErrNoMorePackets.Error())
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
	writePkt(5, true, &ts.PacketAdaptationField{RandomAccessIndicator: true, HasPCR: true, PCR: packetAdaptationField.PCR}, p[:33])
	writePkt(6, false, nil, p[33:])
	// Second unit start flushes the first; it drains at EOF itself
	writePkt(7, true, nil, p)

	dmx := New(context.Background(), bytes.NewReader(buf.Bytes()), WithPacketSize(ts.PacketSize))

	ev, err := dmx.Next()
	require.NoError(t, err)
	require.Equal(t, EventPES, ev)
	first := dmx.PES()
	require.NotNil(t, first)
	assert.Equal(t, uint16(256), first.PID)
	assert.Equal(t, uint8(5), first.ContinuityCounter, "CC of the unit's first packet")
	require.NotNil(t, first.AdaptationField)
	assert.True(t, first.AdaptationField.RandomAccessIndicator)
	assert.Equal(t, packetAdaptationField.PCR.Base(), first.AdaptationField.PCR.Base())

	var wantData pes.Data
	require.NoError(t, wantData.Parse(p))
	assert.Equal(t, wantData.Data, first.Data.Data)
	first.Close()

	ev, err = dmx.Next()
	require.NoError(t, err)
	require.Equal(t, EventPES, ev)
	_, err = dmx.Next()
	assert.EqualError(t, err, ts.ErrNoMorePackets.Error())
	dmx.Close()
}

// The flag follows the declared length against the bytes present, and only
// for a drained unit: one closed by the next unit start is parsed strictly.
func TestDemuxerDrainTruncated(t *testing.T) {
	const payload = "payload"
	const optionalHeader = 3
	pesBytes := func(length uint16) []byte {
		b := []byte{0, 0, 1, 0xc0, byte(length >> 8), byte(length), 0x80, 0, 0}
		return append(b, payload...)
	}
	present := len(padPayload(nil)) - pes.HeaderSize - optionalHeader
	whole := uint16(optionalHeader + len(payload))
	short := uint16(optionalHeader + len(payload) + 200)

	tests := []struct {
		name      string
		length    uint16
		closed    bool // a following unit start closes the unit instead of the drain
		wantErr   bool
		truncated bool
		dataLen   int
	}{
		{"drained, bounded and whole", whole, false, false, false, len(payload)},
		{"drained, bounded and short", short, false, false, true, present},
		{"drained, unbounded", 0, false, false, true, present},
		{"closed, unbounded", 0, true, false, false, present},
		{"closed, bounded and short", short, true, true, false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := packetBytes(ts.PacketHeader{PID: 256, PayloadUnitStartIndicator: true}, pesBytes(tt.length), false)
			if tt.closed {
				next := packetBytes(ts.PacketHeader{PID: 256, ContinuityCounter: 1, PayloadUnitStartIndicator: true}, pesBytes(whole), false)
				stream = append(stream, next...)
			}
			dmx := New(context.Background(), bytes.NewReader(stream), WithPacketSize(ts.PacketSize), WithRecoverableErrors())
			defer dmx.Close()

			ev, err := dmx.Next()
			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, EventError, ev)
				var re *ts.RecoverableError
				require.ErrorAs(t, err, &re)
				assert.Equal(t, ts.ErrorKindPES, re.Kind)
				return
			}
			require.NoError(t, err)
			require.Equal(t, EventPES, ev)
			p := dmx.PES()
			assert.Equal(t, tt.truncated, p.Truncated)
			assert.Len(t, p.Data.Data, tt.dataLen)
		})
	}
}

func TestDemuxerRewind(t *testing.T) {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	b := psiBytes()
	b1 := packetBytes(ts.PacketHeader{ContinuityCounter: uint8(0), PayloadUnitStartIndicator: true, PID: ts.PIDPAT}, b[:147], true)
	_ = w.Write(b1)
	b2 := packetBytes(ts.PacketHeader{ContinuityCounter: uint8(1), PID: ts.PIDPAT}, b[147:], true)
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

	_, err := dmx.Rewind()
	require.NoError(t, err)
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
	b1 := packetBytes(ts.PacketHeader{ContinuityCounter: uint8(0), PayloadUnitStartIndicator: true, PID: ts.PIDPAT}, bs[:147], true)
	_ = w.Write(b1)
	b2 := packetBytes(ts.PacketHeader{ContinuityCounter: uint8(1), PID: ts.PIDPAT}, bs[147:], true)
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
	b1 := packetBytes(ts.PacketHeader{ContinuityCounter: uint8(0), PayloadUnitStartIndicator: true, PID: ts.PIDPAT}, bs[:147], true)
	_ = w.Write(b1)
	b2 := packetBytes(ts.PacketHeader{ContinuityCounter: uint8(1), PID: ts.PIDPAT}, bs[147:], true)
	_ = w.Write(b2)
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
		b1 := packetBytes(ts.PacketHeader{ContinuityCounter: cc, PayloadUnitStartIndicator: true, PID: ts.PIDPAT}, b[:147], true)
		_ = w.Write(b1)
		cc++
		b2 := packetBytes(ts.PacketHeader{ContinuityCounter: cc, PID: ts.PIDPAT}, b[147:], true)
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

// A reader that cannot seek continues after Rewind from the packet the
// demuxer had reached: the window read ahead of it is not replayed.
func TestRewindNonSeekableContinues(t *testing.T) {
	var raw []byte
	for i := range 10 {
		raw = append(raw, payloadPacket(0x100, uint8(i), false, []byte{byte(i)})...)
	}
	dmx := New(context.Background(), struct{ io.Reader }{bytes.NewReader(raw)}, WithPacketSize(ts.PacketSize))
	defer dmx.Close()
	p := ts.NewPacket()
	defer p.Close()
	for range 5 {
		require.NoError(t, dmx.NextPacketTo(p))
	}
	n, err := dmx.Rewind()
	require.NoError(t, err)
	assert.Equal(t, int64(-1), n)
	require.NoError(t, dmx.NextPacketTo(p))
	assert.Equal(t, byte(5), p.Payload[0])
}
