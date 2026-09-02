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
	EventTDT
	EventCAT
	EventBAT
	EventRST
	EventDIT
	EventSIT
	EventST
	EventTSDT
	// EventError: a recoverable parse error was skipped; Next returns it in err
	// (a *ts.RecoverableError) and iteration continues on the following call.
	// Emitted only under WithRecoverableErrors.
	EventError
)

// Demuxer represents a demuxer
// https://en.wikipedia.org/wiki/MPEG_transport_stream
// http://seidl.cs.vsb.cz/download/dvb/DVB_Poster.pdf
// http://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.13.01_40/en_300468v011301o.pdf
type Demuxer struct {
	ctx context.Context
	r   io.Reader

	optPacketSize    uint
	optSkipErrLimit  int
	optResyncLimit   int
	optPacketSkipper ts.PacketSkipper
	optKeepPIDs      *ts.PIDSet
	optZeroCopyBatch uint
	optSyncLock      bool
	optDVBTables     bool
	optPSIRepeats    bool
	optRecoverable   bool
	optPacketHook    func(*ts.Packet)
	optMaxPES        int
	optMaxPSI        int

	packetBuffer *ts.PacketBuffer
	acc          accumulator
	programMap   pidmap.Map[uint16]
	psiPrev      pidmap.Map[psiCache]

	// Result of the last Next
	pat         *psi.PAT
	pmt         *psi.PMT
	patVersion  uint8
	patSeen     bool
	cur         tableEvent // section + changed flag behind the last table event
	tblQueue    []tableEvent
	pendingErrs []*ts.RecoverableError
	// pendingFatal holds a fatal read error until the recoverable errors queued
	// during that same read are flushed, so a lossy tail is not lost to the fatal.
	pendingFatal error
	// pending is the unit behind the last EventPES; claimed once PES() handed
	// it out, so the next Next releases it only while unclaimed.
	pending *PES
	claimed bool

	pkt ts.Packet

	// Inline storage, each paired with a field above to keep the common small
	// case off the heap.
	tblArr     [8]tableEvent           // tblQueue
	errArr     [4]*ts.RecoverableError // pendingErrs
	unitsArr   [2]unit                 // acc.add result
	pmKeysArr  [4]uint16               // programMap keys
	pmValsArr  [4]uint16               // programMap vals
	psiKeysArr [8]uint16               // psiPrev keys
	psiValsArr [8]psiCache             // psiPrev vals
}

// Unit size limits (WithMaxUnitSize): a 4K intra frame at 50 Mbit/s is 2-4 MB
// of unbounded PES; a legal PSI section is at most 4 KB, so 64 KB holds any
// chain of sections up to its stuffing with room to spare.
const (
	defaultMaxPESUnit = 16 << 20
	defaultMaxPSIUnit = 64 << 10
)

// New creates a new transport stream demuxer based on a reader
func New(ctx context.Context, r io.Reader, opts ...func(*Demuxer)) (d *Demuxer) {
	d = &Demuxer{
		ctx:       ctx,
		r:         r,
		optMaxPES: defaultMaxPESUnit,
		optMaxPSI: defaultMaxPSIUnit,
	}
	d.programMap = pidmap.Map[uint16]{Keys: d.pmKeysArr[:0], Vals: d.pmValsArr[:0]}
	d.psiPrev = pidmap.Map[psiCache]{Keys: d.psiKeysArr[:0], Vals: d.psiValsArr[:0]}
	d.tblQueue = d.tblArr[:0]
	d.pendingErrs = d.errArr[:0]

	for _, opt := range opts {
		opt(d)
	}

	d.acc.init(&d.programMap, d.optDVBTables, d.recoverHook(), d.optMaxPES, d.optMaxPSI)

	return
}

// PacketCounts returns the number of packets per PID that reached unit
// assembly: every packet the packet buffer delivered (null and
// adaptation-field-only ones included), after the PID allow-list, the skipper
// and the dropped or reserved packets.
func (dmx *Demuxer) PacketCounts() (ret map[uint16]uint64) {
	ret = make(map[uint16]uint64, len(dmx.acc.slots.Vals))
	for i := range dmx.acc.slots.Vals {
		if n := dmx.acc.slots.Vals[i].packets; n > 0 {
			ret[dmx.acc.slots.Keys[i]] = n
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

// WithKeepPIDs sets an inline PID allow-list checked in the packet parse hot
// path (cheaper than a PacketSkipper call). nil keeps all PIDs.
//
// Filtered packets never reach PSI processing, so include PID 0 (PAT) and the
// PMT PID(s) whenever program or table info is still needed — otherwise the
// demuxer cannot resolve the program map.
func WithKeepPIDs(keep *ts.PIDSet) func(*Demuxer) {
	return func(d *Demuxer) {
		d.optKeepPIDs = keep
	}
}

// SetKeepPIDs installs the inline PID allow-list after construction (see
// WithKeepPIDs for the PAT/PMT caveat). It takes effect on the next packet
// buffer, so set it before the pass that should filter (e.g. after Rewind).
func (dmx *Demuxer) SetKeepPIDs(keep *ts.PIDSet) {
	dmx.optKeepPIDs = keep
}

// WithSkipErrLimit bounds the streak of consecutive damage events before the
// next one is fatal: a packet that fails to parse at an aligned position (it is
// dropped, in either mode) and, under WithSyncLock, a lost sync — counted once
// when the loss is detected, whatever the resync's outcome. A clean packet
// resets the streak; a repaired sync byte and a reserved
// adaptation_field_control packet cost nothing. 0 (the default) tolerates
// nothing, -1 never gives up, N allows N in a row.
func WithSkipErrLimit(count int) func(*Demuxer) {
	return func(d *Demuxer) {
		d.optSkipErrLimit = count
	}
}

// WithSyncLock aligns to the first sync byte within the first scan window (a
// leading header or a mid-stream join, not just a unit boundary; the bytes
// before it are reported as a loss) and keeps the stream alive across damage —
// for UDP/RTP or otherwise torn feeds: a lone corrupt sync byte with the next
// one intact is repaired and reported, not lost (TR 101 290 sync_byte_error);
// two in a row are a sync loss, re-locked only on five consecutive periods
// (TS_sync_loss), so a tail shorter than five packets after a loss is consumed
// into it; an aligned corrupt packet is dropped. How much of that is tolerated
// is set explicitly: the default WithSkipErrLimit makes the first drop or loss
// fatal and the default WithResyncLimit forbids scanning, so a lossy feed needs
// both. It peeks ahead through a ts.Peeker of at least 1024 bytes; a reader
// that is not one (*bufio.Reader is) gets wrapped in bufio internally. Keep it
// off for aligned files to stay on the zero-wrap fast path.
func WithSyncLock() func(*Demuxer) {
	return func(d *Demuxer) {
		d.optSyncLock = true
	}
}

// WithResyncLimit is how many scan windows (about a kilobyte each) one resync
// may spend re-locking after a sync loss under WithSyncLock: 0 (the default)
// makes a lost sync fatal at once, -1 scans to the end of input, N gives up
// after N fruitless windows. Every loss starts a fresh count; the loss itself
// also counts once toward WithSkipErrLimit. Has no effect without WithSyncLock.
func WithResyncLimit(windows int) func(*Demuxer) {
	return func(d *Demuxer) {
		d.optResyncLimit = windows
	}
}

// WithMaxUnitSize caps the bytes one payload unit may accumulate, separately
// for PES and PSI units, on the shared limit scale: 0 delivers no unit of that
// kind, -1 is unbounded, N is the byte limit. A unit outgrowing its limit is
// torn (an ErrorKindTornUnit event with ts.ErrUnitTooLarge under
// WithRecoverableErrors) and the next payload packet on its PID opens a new
// unit, so a PID whose unit start never comes cannot buffer the rest of the
// stream. The defaults are 16 MB for PES and 64 KB for PSI.
func WithMaxUnitSize(pes, psi int) func(*Demuxer) {
	return func(d *Demuxer) {
		d.optMaxPES = pes
		d.optMaxPSI = psi
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

// WithDVBTables enables parsing of the DVB tables (EIT/NIT/SDT/TOT/TDT ranges);
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

// WithPacketHook runs fn on every raw packet as it is read, before unit
// assembly, letting one traversal serve both packet- and unit-level work. The
// packet is valid only for the duration of the call.
func WithPacketHook(fn func(*ts.Packet)) func(*Demuxer) {
	return func(d *Demuxer) {
		d.optPacketHook = fn
	}
}

// WithRecoverableErrors surfaces non-fatal damage the demuxer would otherwise
// skip silently: a PSI CRC32 mismatch, a torn PSI section, a bad PES unit, a
// lost sync (with the bytes scanned past) or a lone repaired sync byte, a
// dropped corrupt packet, a unit torn by a continuity gap, a discontinuity, a
// transport error or scrambling, and a unit that is neither PES nor PSI. Next
// then returns EventError with a *ts.RecoverableError in err (kind, PID,
// offset, bytes dropped) and continues on the following call; Events yields it
// without ending the sequence. Off by default: no events, no calls on the hot
// path; the damage handling itself is the same either way.
func WithRecoverableErrors() func(*Demuxer) {
	return func(d *Demuxer) {
		d.optRecoverable = true
	}
}

func (dmx *Demuxer) reportRecoverable(e ts.RecoverableError) {
	dmx.pendingErrs = append(dmx.pendingErrs, &e)
}

// nil without WithRecoverableErrors keeps the packet buffer and accumulator call-free.
func (dmx *Demuxer) recoverHook() (hook func(ts.RecoverableError)) {
	if dmx.optRecoverable {
		hook = dmx.reportRecoverable
	}
	return
}

func isCancel(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func (dmx *Demuxer) nextPacket(p *ts.Packet) (err error) {
	if dmx.packetBuffer == nil {
		if dmx.packetBuffer, err = ts.NewPacketBuffer(dmx.ctx, dmx.r, ts.PacketBufferConfig{
			PacketSize:    dmx.optPacketSize,
			SkipErrLimit:  dmx.optSkipErrLimit,
			Skipper:       dmx.optPacketSkipper,
			KeepPIDs:      dmx.optKeepPIDs,
			ZeroCopyBatch: dmx.optZeroCopyBatch,
			SyncLock:      dmx.optSyncLock,
			ResyncLimit:   dmx.optResyncLimit,
			OnRecover:     dmx.recoverHook(),
		}); err != nil {
			err = fmt.Errorf("astits: creating packet buffer failed: %w", err)
			return
		}
	}

	if err = dmx.packetBuffer.Next(p); err != nil {
		if !errors.Is(err, ts.ErrNoMorePackets) && !isCancel(err) {
			err = fmt.Errorf("astits: fetching next packet from buffer failed: %w", err)
		}
		return
	}
	if dmx.optPacketHook != nil {
		dmx.optPacketHook(p)
	}
	return
}

// NextPacket retrieves the next packet. You must Close() the packet after use.
func (dmx *Demuxer) NextPacket() (p *ts.Packet, err error) {
	p = ts.NewPacket()

	if err = dmx.NextPacketTo(p); err != nil {
		p.Close()
		p = nil
	}

	return
}

// NextPacketTo unpack packet to provided p.
func (dmx *Demuxer) NextPacketTo(p *ts.Packet) (err error) {
	return dmx.nextPacket(p)
}

// Next advances the demuxer to the next event. On EventPES claim the unit via
// PES(); an unclaimed unit is released by the following Next. On EventTable
// see Section() and the PAT()/PMT() state. EOF is ts.ErrNoMorePackets; the
// unfinished unit tails are emitted before it in ascending PID order.
func (dmx *Demuxer) Next() (ev Event, err error) {
	// Release an unclaimed unit of the previous event
	if dmx.pending != nil {
		if !dmx.claimed {
			dmx.pending.Close()
		}
		dmx.pending = nil
		dmx.claimed = false
	}

	for {
		// Recoverable errors reported by the packet buffer or unit parsing come
		// out first, one per call; EventError carries them in err.
		if len(dmx.pendingErrs) > 0 {
			e := dmx.pendingErrs[0]
			dmx.pendingErrs = dmx.pendingErrs[1:]
			if len(dmx.pendingErrs) == 0 {
				dmx.pendingErrs = dmx.errArr[:0]
			}
			return EventError, e
		}
		if dmx.pendingFatal != nil {
			err = dmx.pendingFatal
			dmx.pendingFatal = nil
			return 0, err
		}

		// Queued table emissions next
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
			if isCancel(err) {
				return 0, err
			}
			if !errors.Is(err, ts.ErrNoMorePackets) {
				werr := fmt.Errorf("astits: fetching next packet failed: %w", err)
				// Flush recoverable errors reported during this failed read before
				// the fatal one ends the stream.
				if len(dmx.pendingErrs) > 0 {
					dmx.pendingFatal = werr
					continue
				}
				return 0, werr
			}
			// EOF: the errors of the final read come out before the drained
			// tails, then the unfinished units, lowest PID first. The reader is
			// retried on the next call — it may grow.
			if len(dmx.pendingErrs) > 0 {
				continue
			}
			u, ok := dmx.acc.drain()
			if !ok {
				if len(dmx.pendingErrs) > 0 {
					continue
				}
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
// Next call: its header (table_id), syntax header (version, current_next,
// section numbers) and the typed body in Syntax.Data. Tables without a syntax
// header (TDT/TOT/RST/ST/DIT/Metadata) leave Syntax.Header zero. nil before
// the first table event; after another kind of event it still holds the last
// table event's section.
func (dmx *Demuxer) Section() (pid uint16, s *psi.Section) {
	return dmx.cur.pid, dmx.cur.sec
}

// TableChanged reports whether the last table event carried content that
// differs from the previous occurrence on its PID. Always true unless
// WithPSIRepeats is set, which also emits events for byte-identical repeats
// (then false). Valid at a table event.
func (dmx *Demuxer) TableChanged() bool {
	return dmx.cur.changed
}

// PAT is the last program association table in effect (current_next_indicator
// set); nil until one is seen.
func (dmx *Demuxer) PAT() *psi.PAT {
	return dmx.pat
}

// PMT is the last program map table in effect (current_next_indicator set);
// nil until one is seen.
func (dmx *Demuxer) PMT() *psi.PMT {
	return dmx.pmt
}

// Events iterates Next until the packets are exhausted: ts.ErrNoMorePackets
// ends the sequence, a recoverable error (EventError, see WithRecoverableErrors)
// is yielded and iteration continues, any other error is yielded once and ends
// the sequence.
func (dmx *Demuxer) Events() iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		for {
			ev, err := dmx.Next()
			if err != nil {
				if errors.Is(err, ts.ErrNoMorePackets) {
					return
				}
				if !yield(ev, err) {
					return
				}
				if ts.IsRecoverable(err) {
					continue
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
// the pending unit. Units still accumulating are dropped without an event; to
// account for them, read Next up to ts.ErrNoMorePackets first, which drains
// them. The demuxer must not be used after Close. Mandatory for demuxers
// abandoned before the end of the stream.
func (dmx *Demuxer) Close() {
	if dmx.pending != nil && !dmx.claimed {
		dmx.pending.Close()
	}
	dmx.pending = nil
	dmx.acc.close()
}

// Rewind rewinds the demuxer reader. The table state survives, the emission
// dedup does not: tables are re-emitted on the second pass. Units still
// accumulating and queued events are dropped without an event, as in Close.
func (dmx *Demuxer) Rewind() (n int64, err error) {
	dmx.Close()
	dmx.packetBuffer = nil
	dmx.tblQueue = dmx.tblArr[:0]
	dmx.pendingErrs = dmx.errArr[:0]
	dmx.pendingFatal = nil
	dmx.patSeen = false
	dmx.psiPrev = pidmap.Map[psiCache]{Keys: dmx.psiKeysArr[:0], Vals: dmx.psiValsArr[:0]}
	dmx.acc.init(&dmx.programMap, dmx.optDVBTables, dmx.recoverHook(), dmx.optMaxPES, dmx.optMaxPSI)
	if n, err = ts.Rewind(dmx.r); err != nil {
		err = fmt.Errorf("astits: rewinding reader failed: %w", err)
		return
	}
	return
}
