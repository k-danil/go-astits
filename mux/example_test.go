package mux_test

import (
	"context"
	"io"

	"github.com/k-danil/go-astits/v2/demux"
	"github.com/k-danil/go-astits/v2/mux"
	"github.com/k-danil/go-astits/v2/pes"
	"github.com/k-danil/go-astits/v2/psi"
	"github.com/k-danil/go-astits/v2/ts"
)

// Build a single-program stream: register the elementary streams, write the
// tables, then write PES units.
func ExampleMuxer() {
	var w io.Writer // the output

	m := mux.New(context.Background(), w)
	if err := m.AddElementaryStream(psi.ElementaryStream{
		ElementaryPID: 0x100,
		StreamType:    psi.StreamTypeH264Video,
	}); err != nil {
		return
	}
	m.SetPCRPID(0x100)

	if _, err := m.WriteTables(); err != nil {
		return
	}

	var accessUnit []byte // one access unit's elementary bytes
	_, _ = m.WriteData(&mux.Data{
		PID: 0x100,
		PES: &pes.Data{
			Header: pes.Header{StreamID: 0xe0}, // a video stream id
			Data:   accessUnit,
		},
	})
}

// Passthrough with a PID rewrite: read raw packets, patch the header, and write
// the bytes straight through. Works in zero-copy view mode too, where Raw() is
// a view into the batch buffer.
func ExampleMuxer_WritePacket() {
	var r io.Reader // an MPEG-TS stream
	var w io.Writer // the output

	dmx := demux.New(context.Background(), r, demux.WithPacketSize(ts.PacketSize))
	defer dmx.Close()
	m := mux.New(context.Background(), w)

	p := ts.NewPacket()
	defer p.Close()
	for {
		if err := dmx.NextPacketTo(p); err != nil {
			break
		}
		p.Header.PID = 0x200 // rewrite the PID
		p.UpdateHeader()     // patch the raw header bytes
		if _, err := m.WritePacket(p); err != nil {
			return
		}
	}
}
