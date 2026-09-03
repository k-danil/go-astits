package demux_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/k-danil/go-astits/v3/demux"
	"github.com/k-danil/go-astits/v3/mux"
	"github.com/k-danil/go-astits/v3/psi"
	"github.com/k-danil/go-astits/v3/ts"
	"github.com/k-danil/go-astits/v3/tsio"
)

// The canonical demux loop: advance to each event, claim and release PES units,
// read table state on table events.
func ExampleDemuxer_Next() {
	var r io.Reader // an MPEG-TS stream

	dmx := demux.New(context.Background(), r, demux.WithPacketSize(ts.PacketSize))
	defer dmx.Close()

	for {
		ev, err := dmx.Next()
		if errors.Is(err, ts.ErrNoMorePackets) {
			break // end of stream
		}
		if err != nil {
			return // read or parse error
		}

		switch ev {
		case demux.EventPES:
			p := dmx.PES() // owned until Close
			_ = p.PID      // use p.Data, p.AdaptationField, ...
			p.Close()      // release when done with it
		case demux.EventPMT:
			_ = dmx.PMT() // the program map changed
		}
	}
}

// Events is a range-over-func wrapper around Next: iteration ends on EOF, and a
// non-nil err is yielded for real failures.
func ExampleDemuxer_Events() {
	var r io.Reader // an MPEG-TS stream

	dmx := demux.New(context.Background(), r, demux.WithPacketSize(ts.PacketSize))
	defer dmx.Close()

	for ev, err := range dmx.Events() {
		if err != nil {
			return
		}
		if ev == demux.EventPES {
			p := dmx.PES()
			_ = p.Data
			p.Close()
		}
	}
}

// A claimed PES stays valid across later Next calls until Close, so units can be
// buffered; a unit left unclaimed is released by the next Next.
func ExampleDemuxer_PES() {
	var r io.Reader // an MPEG-TS stream

	dmx := demux.New(context.Background(), r, demux.WithPacketSize(ts.PacketSize))
	defer dmx.Close()

	var buffered []*demux.PES
	for {
		ev, err := dmx.Next()
		if err != nil {
			break
		}
		if ev == demux.EventPES {
			buffered = append(buffered, dmx.PES()) // claim: survives later Next
		}
	}

	for _, p := range buffered {
		_ = p.Data
		p.Close()
	}
}

// A minimal round trip: mux a program's tables, then demux them back.
func Example() {
	var buf bytes.Buffer

	m := mux.New(context.Background(), &buf)
	if err := m.AddElementaryStream(psi.ElementaryStream{
		ElementaryPID: 0x100,
		StreamType:    psi.StreamTypeH264Video,
	}); err != nil {
		panic(err)
	}
	m.SetPCRPID(0x100)
	if _, err := m.WriteTables(); err != nil {
		panic(err)
	}

	dmx := demux.New(context.Background(), &buf, demux.WithPacketSize(ts.PacketSize))
	defer dmx.Close()

	for ev, err := range dmx.Events() {
		if err != nil {
			panic(err)
		}
		if ev == demux.EventPMT {
			fmt.Printf("PMT elementary PID: 0x%x\n", dmx.PMT().ElementaryStreams[0].ElementaryPID)
			break
		}
	}
	// Output: PMT elementary PID: 0x100
}

// A torn live feed (UDP, RTP): lock onto the sync byte at any offset, survive
// damage instead of failing on the first corrupt packet, and hear about it.
func ExampleWithSyncLock() {
	var r io.Reader // a datagram reassembler, or any reader

	dmx := demux.New(context.Background(), r,
		demux.WithSyncLock(),
		demux.WithSkipErrLimit(-1), demux.WithResyncLimit(-1), // never give up
		demux.WithRecoverableErrors())
	defer dmx.Close()

	for ev, err := range dmx.Events() {
		if re, ok := errors.AsType[*ts.RecoverableError](err); ok {
			switch re.Kind { // TR 101 290 counters, for instance
			case ts.ErrorKindSyncLoss:
				_ = re.Dropped // bytes lost until the next lock
			case ts.ErrorKindSyncByte, ts.ErrorKindPacketDrop, ts.ErrorKindCRC:
			}
			continue
		}
		if err != nil {
			return
		}
		if ev == demux.EventPES {
			dmx.PES().Close()
		}
	}
}

// Keep only some PIDs: the allow-list is checked right after the header,
// before any payload work, so the PAT and the PMT PIDs must stay on it or no
// program can be resolved.
func ExampleWithKeepPIDs() {
	var r io.Reader

	keep := ts.NewPIDSet(ts.PIDPAT, 0x1000, 0x100) // PAT, this program's PMT, its video
	dmx := demux.New(context.Background(), r,
		demux.WithPacketSize(ts.PacketSize), demux.WithKeepPIDs(&keep))
	defer dmx.Close()

	for ev, err := range dmx.Events() {
		if err != nil {
			return
		}
		if ev == demux.EventPES {
			dmx.PES().Close() // only PID 0x100 gets here
		}
	}
}

// Packet-level work without a copy per packet: the packet is a view into the
// read window, valid until the next NextPacketTo — copy what must outlive it.
func ExampleWithZeroCopyPackets() {
	var r io.Reader

	dmx := demux.New(context.Background(), r,
		demux.WithPacketSize(ts.PacketSize), demux.WithZeroCopyPackets(1024))
	defer dmx.Close()

	p := ts.NewPacket()
	defer p.Close()
	var kept [][]byte
	for {
		if err := dmx.NextPacketTo(p); err != nil {
			break
		}
		if p.Header.PayloadUnitStartIndicator {
			kept = append(kept, append([]byte(nil), p.Payload...)) // owned copy
		}
	}
	_ = kept
}

// Two passes over a file: a short prefix scan for the program map, then a
// rewind and the real pass. Behind a tsio.SeekBuffer the rewind stays inside
// the buffered window and costs no I/O.
func ExampleDemuxer_Rewind() {
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
		if ev == demux.EventPMT {
			break // the program map is known
		}
	}

	if _, err = dmx.Rewind(); err != nil {
		return
	}
	for ev, err := range dmx.Events() { // tables re-emit, the map survives
		if err != nil {
			return
		}
		if ev == demux.EventPES {
			dmx.PES().Close()
		}
	}
}

// DVB service information: with WithDVBTables the SI tables are parsed too,
// and Section hands over the one behind the last table event.
func ExampleDemuxer_Section() {
	var r io.Reader

	dmx := demux.New(context.Background(), r,
		demux.WithPacketSize(ts.PacketSize), demux.WithDVBTables())
	defer dmx.Close()

	for ev, err := range dmx.Events() {
		if err != nil {
			return
		}
		switch ev {
		case demux.EventSDT, demux.EventEIT:
			pid, s := dmx.Section() // valid until the next Next
			switch table := s.Syntax.Data.(type) {
			case *psi.SDT:
				_, _ = pid, table.Services
			case *psi.EIT:
				_, _ = pid, table.Events
			}
		case demux.EventPES:
			dmx.PES().Close()
		}
	}
}

// Where a table came from: the span covers the packets its unit was assembled
// from, so a section can be located in the stream or charged to the datagram
// that carried it.
func ExampleDemuxer_SectionSpan() {
	var r io.Reader

	dmx := demux.New(context.Background(), r, demux.WithPacketSize(ts.PacketSize))
	defer dmx.Close()

	for ev, err := range dmx.Events() {
		if err != nil {
			return
		}
		switch ev {
		case demux.EventPMT:
			span := dmx.SectionSpan()
			fmt.Printf("PMT over packets %d..%d\n", span.FirstPacketOffset, span.LastPacketOffset)
		case demux.EventPES:
			dmx.PES().Close()
		}
	}
}

// Per-packet accounting alongside the event loop: the hook sees every packet
// that reaches unit assembly, for the duration of the call only.
func ExampleWithPacketHook() {
	var r io.Reader

	bytesPerPID := map[uint16]int{}
	dmx := demux.New(context.Background(), r,
		demux.WithPacketSize(ts.PacketSize),
		demux.WithPacketHook(func(p *ts.Packet) { bytesPerPID[p.Header.PID] += len(p.Raw()) }),
		demux.WithMaxUnitSize(4<<20, 64<<10)) // cap a runaway PES at 4 MB, a PSI unit at 64 KB
	defer dmx.Close()

	for ev, err := range dmx.Events() {
		if err != nil {
			return
		}
		if ev == demux.EventPES {
			dmx.PES().Close()
		}
	}
	_ = bytesPerPID
}
