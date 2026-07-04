package demux_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/k-danil/go-astits/v2/demux"
	"github.com/k-danil/go-astits/v2/mux"
	"github.com/k-danil/go-astits/v2/psi"
	"github.com/k-danil/go-astits/v2/ts"
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
