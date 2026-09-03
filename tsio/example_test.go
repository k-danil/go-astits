package tsio_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/k-danil/go-astits/v3/demux"
	"github.com/k-danil/go-astits/v3/mux"
	"github.com/k-danil/go-astits/v3/pes"
	"github.com/k-danil/go-astits/v3/psi"
	"github.com/k-danil/go-astits/v3/ts"
	"github.com/k-danil/go-astits/v3/tsio"
)

// A file read through a bounded buffer: packets are views into it, and a Seek
// back inside the buffered window (a demuxer Rewind after a prefix scan)
// touches neither the file nor the buffer.
func ExampleNewSeekBuffer() {
	f, err := os.Open("stream.ts")
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	sb := tsio.NewSeekBuffer(4 << 20)
	if err = sb.Reset(f); err != nil {
		return
	}
	dmx := demux.New(context.Background(), sb, demux.WithPacketSize(ts.PacketSize))
	defer dmx.Close()
	for ev, err := range dmx.Events() {
		if err != nil {
			return
		}
		if ev == demux.EventPES {
			dmx.PES().Close()
		}
	}
}

// oneProgram muxes a PAT, a PMT and one PES unit into memory.
func oneProgram() []byte {
	var buf bytes.Buffer
	m := mux.New(context.Background(), &buf)
	if err := m.AddElementaryStream(psi.ElementaryStream{ElementaryPID: 0x100, StreamType: psi.StreamTypeH264Video}); err != nil {
		panic(err)
	}
	m.SetPCRPID(0x100)
	if _, err := m.WriteData(&mux.Data{PID: 0x100, PES: &pes.Data{
		Header: pes.Header{StreamID: 0xe0, OptionalHeader: &pes.OptionalHeader{PTSDTSIndicator: pes.PTSDTSIndicatorOnlyPTS, PTS: ts.NewClockReference(90000, 0)}},
		Data:   []byte("access unit"),
	}}); err != nil {
		panic(err)
	}
	return buf.Bytes() // PAT, PMT, then the PES packet
}

// A chunk already in memory (or mmap'd) is served without any copy on the way
// to the parser.
func ExampleNewBytesReader_demux() {
	chunk := oneProgram()

	dmx := demux.New(context.Background(), tsio.NewBytesReader(chunk), demux.WithPacketSize(ts.PacketSize))
	defer dmx.Close()
	units := 0
	for ev, err := range dmx.Events() {
		if err != nil {
			panic(err)
		}
		if ev == demux.EventPES {
			units++
			dmx.PES().Close()
		}
	}
	fmt.Println(units, "PES unit(s) from", len(chunk)/ts.PacketSize, "packets")
	// Output: 1 PES unit(s) from 3 packets
}

// frameReader serves a stream in two-packet frames and stamps each frame with
// its index, the way a datagram source would stamp a receive timestamp:
// Buffered stops at the frame boundary, so a window never spans two tags.
const frameBytes = 2 * ts.PacketSize

type frameReader struct {
	data []byte
	pos  int
}

func (r *frameReader) frameEnd() int {
	return min(r.pos/ts.PacketSize*ts.PacketSize+ts.PacketSize, len(r.data))
}
func (r *frameReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:r.frameEnd()])
	r.pos += n
	return
}
func (r *frameReader) Peek(n int) ([]byte, error) {
	bs := r.data[r.pos:min(r.pos+n, len(r.data))]
	if len(bs) < n {
		return bs, io.EOF
	}
	return bs, nil
}
func (r *frameReader) Discard(n int) (int, error) {
	n = min(n, len(r.data)-r.pos)
	r.pos += n
	return n, nil
}
func (r *frameReader) Buffered() int { return r.frameEnd() - r.pos }
func (r *frameReader) Size() int     { return frameBytes }
func (r *frameReader) Tag() uint64   { return uint64(r.pos/frameBytes) + 1 }

var _ tsio.Tagger = (*frameReader)(nil)

// A reader that implements Tagger stamps every packet: the tag is opaque to the
// library and comes back on Packet.Tag and on PES.FirstPacketTag/LastPacketTag.
func ExampleTagger() {
	dmx := demux.New(context.Background(), &frameReader{data: oneProgram()}, demux.WithPacketSize(ts.PacketSize))
	defer dmx.Close()

	p := ts.NewPacket()
	defer p.Close()
	for dmx.NextPacketTo(p) == nil {
		fmt.Printf("pid 0x%x tag %d\n", p.Header.PID, p.Tag)
	}
	// Output:
	// pid 0x0 tag 1
	// pid 0x1000 tag 1
	// pid 0x100 tag 2
}
