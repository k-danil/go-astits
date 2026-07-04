package demux

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"

	"github.com/k-danil/go-astits/v2/internal/bitstest"
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
	pp := func(pl *ts.PacketList) (ds []*Data, skip bool, err error) { return }
	sp := func(p *ts.Packet) bool { return true }
	dmx := New(context.Background(), nil, WithPacketSize(ps), WithPacketsParser(pp), WithPacketSkipper(sp))
	assert.Equal(t, uint(ps), dmx.optPacketSize)
	assert.Equal(t, fmt.Sprintf("%p", pp), fmt.Sprintf("%p", dmx.optPacketsParser))
	assert.Equal(t, fmt.Sprintf("%p", sp), fmt.Sprintf("%p", dmx.optPacketSkipper))
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
	w.Write(b1)
	b2, p2 := packet(packetHeader, packetAdaptationField, []byte("2"), true)
	p2.Offset = int64(len(b1))
	w.Write(b2)
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

func TestDemuxerNextData(t *testing.T) {
	// Init
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	b := psiBytes()
	b1, _ := packet(ts.PacketHeader{ContinuityCounter: uint8(0), PayloadUnitStartIndicator: true, PID: ts.PIDPAT}, &ts.PacketAdaptationField{}, b[:147], true)
	w.Write(b1)
	b2, _ := packet(ts.PacketHeader{ContinuityCounter: uint8(1), PID: ts.PIDPAT}, &ts.PacketAdaptationField{}, b[147:], true)
	w.Write(b2)
	dmx := New(context.Background(), bytes.NewReader(buf.Bytes()))
	p, err := dmx.NextPacket()
	assert.NoError(t, err)
	_, err = dmx.Rewind()
	assert.NoError(t, err)

	// Next data
	psiData, err := psi.Parse(b)
	assert.NoError(t, err)
	var ds []*Data
	for _, s := range psiData.Sections {
		if !s.Header.TableID.IsUnknown() {
			d, err := dmx.NextData()
			assert.NoError(t, err)
			ds = append(ds, d)
		}
	}
	want := psiToData(psiData, p.AdaptationField, ts.PIDPAT)
	for _, d := range want {
		d.setAdaptationField(p.AdaptationField)
	}
	assert.Equal(t, want, ds)
	assert.Equal(t, []uint16{0x3, 0x5}, dmx.programMap.Keys)
	assert.Equal(t, []uint16{0x2, 0x4}, dmx.programMap.Vals)

	// No more packets
	_, err = dmx.NextData()
	assert.EqualError(t, err, ts.ErrNoMorePackets.Error())
}

func TestDemuxerNextDataUnknownDataPackets(t *testing.T) {
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

	// The demuxer must return "no more packets"
	dmx := New(context.Background(), bytes.NewReader(buf.Bytes()),
		WithPacketSize(188))
	d, err := dmx.NextData()
	assert.Equal(t, (*Data)(nil), d)
	assert.EqualError(t, err, ts.ErrNoMorePackets.Error())
}

func TestDemuxerNextDataPATPMT(t *testing.T) {
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

	d, err := dmx.NextData()
	assert.NoError(t, err)
	assert.Equal(t, uint16(0), d.PID)
	assert.NotNil(t, d.PAT)
	assert.Equal(t, 188, r.Len())

	d, err = dmx.NextData()
	assert.NoError(t, err)
	assert.Equal(t, uint16(0x1000), d.PID)
	assert.NotNil(t, d.PMT)
}

func TestDemuxerRewind(t *testing.T) {
	r := bytes.NewReader([]byte("content"))
	dmx := New(context.Background(), r)
	dmx.packetPool.addUnlocked(&ts.Packet{Header: ts.PacketHeader{PID: 1}})
	dmx.dataBuffer = append(dmx.dataBuffer, &Data{})
	b := make([]byte, 2)
	_, err := r.Read(b)
	assert.NoError(t, err)
	n, err := dmx.Rewind()
	assert.NoError(t, err)
	assert.Equal(t, int64(0), n)
	assert.Equal(t, 7, r.Len())
	assert.Equal(t, 0, len(dmx.dataBuffer))
	for i := range dmx.packetPool.slots.Vals {
		assert.Nil(t, dmx.packetPool.slots.Vals[i].q)
	}
	assert.Nil(t, dmx.packetBuffer)
}

func BenchmarkDemuxer_NextData(b *testing.B) {
	b.ReportAllocs()

	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	bs := psiBytes()
	b1, _ := packet(ts.PacketHeader{ContinuityCounter: uint8(0), PayloadUnitStartIndicator: true, PID: ts.PIDPAT}, &ts.PacketAdaptationField{}, bs[:147], true)
	w.Write(b1)
	b2, _ := packet(ts.PacketHeader{ContinuityCounter: uint8(1), PID: ts.PIDPAT}, &ts.PacketAdaptationField{}, bs[147:], true)
	w.Write(b2)

	r := bytes.NewReader(buf.Bytes())
	dmx := New(context.Background(), r)

	psiData, err := psi.Parse(bs)
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < b.N; i++ {
		r.Seek(0, io.SeekStart)
		for _, s := range psiData.Sections {
			if !s.Header.TableID.IsUnknown() {
				dmx.NextData()
			}
		}
	}
}

func FuzzDemuxer(f *testing.F) {
	f.Fuzz(func(t *testing.T, b []byte) {
		r := bytes.NewReader(b)
		dmx := New(context.Background(), r, WithPacketSize(188))
		for {
			_, err := dmx.NextData()
			if err == ts.ErrNoMorePackets {
				break
			}
		}
	})
}
