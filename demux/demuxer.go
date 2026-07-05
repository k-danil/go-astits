package demux

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"

	"github.com/k-danil/go-astits/v2/internal/pidmap"
	"github.com/k-danil/go-astits/v2/psi"
	"github.com/k-danil/go-astits/v2/ts"
)

// Event is what a Next call advanced to. Every table event carries its PSI
// type; the payload is behind Section() (and PAT()/PMT() for those two).
type Event uint8

const (
	// EventPES: a PES unit completed; claim it via Demuxer.PES().
	EventPES Event = iota
	EventPAT
	EventPMT
	EventNIT
	EventSDT
	EventTOT
	EventEIT
)

// Demuxer represents a demuxer
// https://en.wikipedia.org/wiki/MPEG_transport_stream
// http://seidl.cs.vsb.cz/download/dvb/DVB_Poster.pdf
// http://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.13.01_40/en_300468v011301o.pdf
type Demuxer struct {
	ctx context.Context
	r   io.Reader

	optPacketSize    uint
	optSkipErrLimit  uint
	optPacketSkipper ts.PacketSkipper
	optZeroCopyBatch uint
	optDVBTables     bool
	optPSIRepeats    bool

	packetBuffer *ts.PacketBuffer
	acc          accumulator
	programMap   pidmap.Map[uint16]
	psiPrev      pidmap.Map[psiCache]

	// Result of the last Next
	pat      *psi.PAT
	pmt      *psi.PMT
	cur      tableEvent // section + changed flag behind the last table event
	tblQueue []tableEvent
	pending  *PES
	claimed  bool

	pkt ts.Packet

	// Inline storage, each paired with a field above to keep the common small
	// case off the heap.
	tblArr     [8]tableEvent // tblQueue
	unitsArr   [2]unit       // acc.add result
	pmKeysArr  [4]uint16     // programMap keys
	pmValsArr  [4]uint16     // programMap vals
	psiKeysArr [8]uint16     // psiPrev keys
	psiValsArr [8]psiCache   // psiPrev vals
}

// New creates a new transport stream demuxer based on a reader
func New(ctx context.Context, r io.Reader, opts ...func(*Demuxer)) (d *Demuxer) {
	d = &Demuxer{
		ctx:              ctx,
		optPacketSkipper: ts.EmptySkipper,
		r:                r,
	}
	d.programMap = pidmap.Map[uint16]{Keys: d.pmKeysArr[:0], Vals: d.pmValsArr[:0]}
	d.psiPrev = pidmap.Map[psiCache]{Keys: d.psiKeysArr[:0], Vals: d.psiValsArr[:0]}
	d.tblQueue = d.tblArr[:0]

	for _, opt := range opts {
		opt(d)
	}

	d.acc.init(&d.programMap, d.optDVBTables)

	return
}

// GetStats returns the number of stream bytes seen per PID, keyed by PID.
func (dmx *Demuxer) GetStats() (ret map[uint64]uint) {
	var packetSize uint
	if dmx.packetBuffer != nil {
		packetSize = dmx.packetBuffer.PacketSize()
	}

	ret = make(map[uint64]uint, len(dmx.acc.slots.Vals))
	for i := range dmx.acc.slots.Vals {
		if n := dmx.acc.slots.Vals[i].stats; n > 0 {
			ret[uint64(dmx.acc.slots.Keys[i])] = uint(n) * packetSize
		}
	}

	return
}

// WithPacketSize returns the option to set the packet size
func WithPacketSize(packetSize int) func(*Demuxer) {
	return func(d *Demuxer) {
		d.optPacketSize = uint(packetSize)
	}
}

// WithPacketSkipper returns the option to set the packet skipper
func WithPacketSkipper(s ts.PacketSkipper) func(*Demuxer) {
	return func(d *Demuxer) {
		if s != nil {
			d.optPacketSkipper = s
		}
	}
}

// WithSkipErrLimit returns the option to set the tolerated sync-loss streak
func WithSkipErrLimit(count int) func(*Demuxer) {
	return func(d *Demuxer) {
		d.optSkipErrLimit = uint(count)
	}
}

// WithZeroCopyPackets makes packet reads batched: packets are views into the
// internal buffer, valid until the refill triggered by a later read. The
// accumulator copies payloads out immediately, so Next works in this mode.
func WithZeroCopyPackets(batchPackets uint) func(*Demuxer) {
	return func(d *Demuxer) {
		d.optZeroCopyBatch = batchPackets
	}
}

// WithDVBTables enables parsing of the DVB tables (EIT/NIT/SDT/TOT ranges);
// without it only PAT and PMT are parsed.
func WithDVBTables() func(*Demuxer) {
	return func(d *Demuxer) {
		d.optDVBTables = true
	}
}

// WithPSIRepeats emits a table event for every occurrence of a section,
// including byte-identical repeats (TableChanged reports false for those).
// Repeats reuse the cached parse — no re-parse, no allocation. Useful for
// analyzing table insertion cadence. Without it, only content changes emit.
func WithPSIRepeats() func(*Demuxer) {
	return func(d *Demuxer) {
		d.optPSIRepeats = true
	}
}

func (dmx *Demuxer) nextPacket(p *ts.Packet) (err error) {
	if dmx.packetBuffer == nil {
		if dmx.packetBuffer, err = ts.NewPacketBuffer(dmx.r, dmx.optPacketSize, dmx.optSkipErrLimit, dmx.optPacketSkipper, dmx.optZeroCopyBatch); err != nil {
			err = fmt.Errorf("astits: creating packet buffer failed: %w", err)
			return
		}
	}

	if err = dmx.packetBuffer.Next(p); err != nil {
		if err != ts.ErrNoMorePackets {
			err = fmt.Errorf("astits: fetching next packet from buffer failed: %w", err)
		}
	}
	return
}

// NextPacket retrieves the next packet. You must Close() the packet after use.
func (dmx *Demuxer) NextPacket() (p *ts.Packet, err error) {
	p = ts.NewPacket()

	select {
	case <-dmx.ctx.Done():
		err = dmx.ctx.Err()
	default:
		err = dmx.nextPacket(p)
	}

	if err != nil {
		p.Close()
		return nil, err
	}

	return
}

// NextPacketTo unpack packet to provided p.
func (dmx *Demuxer) NextPacketTo(p *ts.Packet) (err error) {
	select {
	case <-dmx.ctx.Done():
		err = dmx.ctx.Err()
	default:
		err = dmx.nextPacket(p)
	}

	return
}

// Next advances the demuxer to the next event. On EventPES claim the unit via
// PES(); an unclaimed unit is released by the following Next. On EventTable
// see Section() and the PAT()/PMT() state. EOF is ts.ErrNoMorePackets; the
// unfinished unit tails are emitted before it in ascending PID order.
func (dmx *Demuxer) Next() (ev Event, err error) {
	select {
	case <-dmx.ctx.Done():
		return 0, dmx.ctx.Err()
	default:
	}

	// Release an unclaimed unit of the previous event
	if dmx.pending != nil {
		if !dmx.claimed {
			dmx.pending.Close()
		}
		dmx.pending = nil
		dmx.claimed = false
	}

	for {
		// Queued table emissions first
		if len(dmx.tblQueue) > 0 {
			e := dmx.tblQueue[0]
			dmx.tblQueue = dmx.tblQueue[1:]
			if len(dmx.tblQueue) == 0 {
				dmx.tblQueue = dmx.tblArr[:0]
			}
			dmx.cur = e
			return e.ev, nil
		}

		var units []unit
		if err = dmx.nextPacket(&dmx.pkt); err != nil {
			if !errors.Is(err, ts.ErrNoMorePackets) {
				return 0, fmt.Errorf("astits: fetching next packet failed: %w", err)
			}
			// EOF: drain the unfinished units, lowest PID first. The reader is
			// retried on the next call — it may grow.
			u, ok := dmx.acc.drain()
			if !ok {
				return 0, ts.ErrNoMorePackets
			}
			units = append(dmx.unitsArr[:0], u)
		} else {
			units = dmx.acc.add(&dmx.pkt, dmx.unitsArr[:0])
		}

		for _, u := range units {
			d, perr := dmx.processUnit(u)
			if perr != nil {
				// A torn or corrupt unit produces no emission
				continue
			}
			if d != nil {
				dmx.pending = d
				dmx.claimed = false
			}
		}
		if dmx.pending != nil {
			return EventPES, nil
		}
	}
}

// PES claims the unit of the last EventPES: the caller owns it until Close.
// An unclaimed unit is released by the next Next call.
func (dmx *Demuxer) PES() *PES {
	if dmx.pending != nil {
		dmx.claimed = true
	}
	return dmx.pending
}

// Section is the section behind the last table event, valid until the next
// Next call.
func (dmx *Demuxer) Section() (pid uint16, s psi.SectionSyntaxData) {
	return dmx.cur.pid, dmx.cur.data
}

// TableChanged reports whether the last table event carried content that
// differs from the previous occurrence on its PID. Always true unless
// WithPSIRepeats is set, which also emits events for byte-identical repeats
// (then false). Valid at a table event.
func (dmx *Demuxer) TableChanged() bool {
	return dmx.cur.changed
}

// PAT is the last parsed program association table; nil until one is seen.
func (dmx *Demuxer) PAT() *psi.PAT {
	return dmx.pat
}

// PMT is the last parsed program map table; nil until one is seen.
func (dmx *Demuxer) PMT() *psi.PMT {
	return dmx.pmt
}

// Events iterates Next until the packets are exhausted: ts.ErrNoMorePackets
// ends the sequence, any other error is yielded with an undefined Event.
func (dmx *Demuxer) Events() iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		for {
			ev, err := dmx.Next()
			if err != nil {
				if !errors.Is(err, ts.ErrNoMorePackets) {
					yield(ev, err)
				}
				return
			}
			if !yield(ev, nil) {
				return
			}
		}
	}
}

// Close releases everything the demuxer holds to the pools: slot buffers and
// the pending unit. The demuxer must not be used after Close. Mandatory for
// demuxers abandoned before the end of the stream.
func (dmx *Demuxer) Close() {
	if dmx.pending != nil && !dmx.claimed {
		dmx.pending.Close()
	}
	dmx.pending = nil
	dmx.acc.close()
}

// Rewind rewinds the demuxer reader. The table state survives, the emission
// dedup does not: tables are re-emitted on the second pass.
func (dmx *Demuxer) Rewind() (n int64, err error) {
	dmx.Close()
	dmx.packetBuffer = nil
	dmx.tblQueue = dmx.tblArr[:0]
	dmx.psiPrev = pidmap.Map[psiCache]{Keys: dmx.psiKeysArr[:0], Vals: dmx.psiValsArr[:0]}
	dmx.acc.init(&dmx.programMap, dmx.optDVBTables)
	if n, err = ts.Rewind(dmx.r); err != nil {
		err = fmt.Errorf("astits: rewinding reader failed: %w", err)
		return
	}
	return
}
