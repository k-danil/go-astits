package demux

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"

	"github.com/k-danil/go-astits/v3/internal/pidmap"
	"github.com/k-danil/go-astits/v3/psi"
	"github.com/k-danil/go-astits/v3/ts"
)

type Event uint8

const (
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
	EventError
)

type Demuxer struct {
	ctx context.Context
	r   io.Reader
	// The read-ahead wrapper is kept apart from r: Rewind seeks r, and a *bufio.Reader is no io.Seeker.
	readAhead io.Reader

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
	keepPIDs     *ts.PIDSet // The allow-list the current packet buffer was built with; walk must use the same one.
	acc          accumulator
	programMap   pidmap.Map[uint16]
	psiPrev      pidmap.Map[psiCache]

	pat         *psi.PAT
	pmt         *psi.PMT
	patVersion  uint8
	patSeen     bool
	cur         tableEvent
	tblQueue    []tableEvent
	pendingErrs []*ts.RecoverableError
	// A fatal read error waits behind the recoverable errors queued during that same read.
	pendingFatal error
	pending      *PES
	claimed      bool

	pkt ts.Packet

	tblArr     [8]tableEvent
	errArr     [4]*ts.RecoverableError
	unitsArr   [2]unit
	pmKeysArr  [4]uint16
	pmValsArr  [4]uint16
	psiKeysArr [8]uint16
	psiValsArr [8]psiCache
}

// A 4K intra frame at 50 Mbit/s is 2-4 MB of unbounded PES; a legal PSI section is at most 4 KB.
const (
	defaultMaxPESUnit = 16 << 20
	defaultMaxPSIUnit = 64 << 10
)

// ctx does not interrupt a Read blocked on r — cancellation is seen between
// reads, so a quiet socket needs a deadline or a Close to wake the demuxer.
// A deadline error is returned wrapped and is not terminal: the next call
// continues from the same position.
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

// Counts what reached unit assembly: after the allow-list, the skipper and dropped packets; null and AF-only packets included.
func (dmx *Demuxer) PacketCounts() (ret map[uint16]uint64) {
	ret = make(map[uint16]uint64, len(dmx.acc.slots.Vals))
	for i := range dmx.acc.slots.Vals {
		ret[dmx.acc.slots.Keys[i]] = dmx.acc.slots.Vals[i].packets
	}
	return
}

func WithPacketSize(packetSize int) func(*Demuxer) {
	return func(d *Demuxer) {
		d.optPacketSize = uint(packetSize)
	}
}

func WithPacketSkipper(s ts.PacketSkipper) func(*Demuxer) {
	return func(d *Demuxer) {
		if s != nil {
			d.optPacketSkipper = s
		}
	}
}

// nil keeps all PIDs. Filtered packets never reach PSI processing: keep PID 0 and the PMT PIDs or the program map cannot resolve.
func WithKeepPIDs(keep *ts.PIDSet) func(*Demuxer) {
	return func(d *Demuxer) {
		d.optKeepPIDs = keep
	}
}

// Takes effect on the next packet buffer, so set it before the pass that should filter (e.g. after Rewind).
func (dmx *Demuxer) SetKeepPIDs(keep *ts.PIDSet) {
	dmx.optKeepPIDs = keep
}

// Bounds the streak of consecutive damage events before the next is fatal; a clean packet resets it. 0 (the default) tolerates nothing, -1 never gives up.
func WithSkipErrLimit(count int) func(*Demuxer) {
	return func(d *Demuxer) {
		d.optSkipErrLimit = count
	}
}

// Aligns to the first sync byte in the first scan window and keeps the stream alive across damage: a lone corrupt sync byte is repaired, two in a row are a sync loss re-locked on five consecutive periods. Tolerance is separate — the default WithSkipErrLimit and WithResyncLimit make the first loss fatal. Needs a tsio.Peeker of at least 1024 bytes; other readers are wrapped in bufio.
func WithSyncLock() func(*Demuxer) {
	return func(d *Demuxer) {
		d.optSyncLock = true
	}
}

// Scan windows (about a kilobyte) one resync may spend re-locking: 0 (the default) makes a lost sync fatal at once, -1 scans to the end. No effect without WithSyncLock.
func WithResyncLimit(windows int) func(*Demuxer) {
	return func(d *Demuxer) {
		d.optResyncLimit = windows
	}
}

// Caps the bytes one unit may accumulate: 0 delivers no unit of that kind, -1 is unbounded. An oversized unit is torn and the next payload packet on its PID opens a new one. Buffers come in power-of-two classes, so a PID holds the limit rounded up.
func WithMaxUnitSize(pes, psi int) func(*Demuxer) {
	return func(d *Demuxer) {
		d.optMaxPES = pes
		d.optMaxPSI = psi
	}
}

// NextPacketTo then hands out packets as views into the read window, valid only until the next read.
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

// Emits a table event for every occurrence of a section, byte-identical repeats included (TableChanged false); without it only content changes emit.
func WithPSIRepeats() func(*Demuxer) {
	return func(d *Demuxer) {
		d.optPSIRepeats = true
	}
}

// The packet is valid only for the duration of the call.
func WithPacketHook(fn func(*ts.Packet)) func(*Demuxer) {
	return func(d *Demuxer) {
		d.optPacketHook = fn
	}
}

// Surfaces non-fatal damage the demuxer otherwise skips: Next returns EventError with a *ts.RecoverableError and continues on the next call. Off by default; the damage handling itself is the same either way.
func WithRecoverableErrors() func(*Demuxer) {
	return func(d *Demuxer) {
		d.optRecoverable = true
	}
}

func (dmx *Demuxer) reportRecoverable(e ts.RecoverableError) {
	dmx.pendingErrs = append(dmx.pendingErrs, &e)
}

func (dmx *Demuxer) recoverHook() (hook func(ts.RecoverableError)) {
	if dmx.optRecoverable {
		hook = dmx.reportRecoverable
	}
	return
}

func isCancel(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func (dmx *Demuxer) ensurePacketBuffer() (err error) {
	if dmx.packetBuffer != nil {
		return
	}
	if dmx.readAhead == nil {
		dmx.readAhead = ts.ReadAheadSize(dmx.r, dmx.optZeroCopyBatch, dmx.optPacketSize)
	}
	dmx.keepPIDs = dmx.optKeepPIDs
	if dmx.packetBuffer, err = ts.NewPacketBuffer(dmx.ctx, dmx.readAhead, ts.PacketBufferConfig{
		PacketSize:    dmx.optPacketSize,
		SkipErrLimit:  dmx.optSkipErrLimit,
		Skipper:       dmx.optPacketSkipper,
		KeepPIDs:      dmx.keepPIDs,
		ZeroCopyBatch: dmx.optZeroCopyBatch,
		SyncLock:      dmx.optSyncLock,
		ResyncLimit:   dmx.optResyncLimit,
		OnRecover:     dmx.recoverHook(),
	}); err != nil {
		err = fmt.Errorf("astits: creating packet buffer failed: %w", err)
	}
	return
}

func wrapReadError(err error) error {
	if errors.Is(err, ts.ErrNoMorePackets) || isCancel(err) {
		return err
	}
	return fmt.Errorf("astits: fetching next packet from buffer failed: %w", err)
}

func (dmx *Demuxer) nextPacket(p *ts.Packet) (err error) {
	if dmx.packetBuffer == nil {
		if err = dmx.ensurePacketBuffer(); err != nil {
			return
		}
	}
	if err = dmx.packetBuffer.Next(p); err != nil {
		return wrapReadError(err)
	}
	if dmx.optPacketHook != nil {
		dmx.optPacketHook(p)
	}
	return
}

// Stops after a packet that completes a unit or queues an error, so a later packet's events cannot overtake it, and before one that fails to parse, which Next handles under its damage budget.
func (dmx *Demuxer) walk(w []byte, out []unit) (units []unit, consumed int, err error) {
	pb := dmx.packetBuffer
	p := &dmx.pkt
	ps := int(pb.PacketSize())
	pos := pb.Pos()
	p.Tag = pb.Tag()
	units = out
	for consumed+ps <= len(w) {
		skip, perr := p.ParseAt(w[consumed:consumed+ps], pos+int64(consumed), dmx.optPacketSkipper, dmx.keepPIDs)
		if perr != nil {
			break
		}
		consumed += ps
		if skip {
			continue
		}
		if dmx.optPacketHook != nil {
			dmx.optPacketHook(p)
		}
		if units = dmx.acc.add(p, units); len(units) > 0 || len(dmx.pendingErrs) > 0 {
			break
		}
	}
	if consumed > 0 {
		err = pb.Advance(consumed)
	}
	return
}

// Close() the returned packet after use.
func (dmx *Demuxer) NextPacket() (p *ts.Packet, err error) {
	p = ts.NewPacket()

	if err = dmx.NextPacketTo(p); err != nil {
		p.Close()
		p = nil
	}

	return
}

func (dmx *Demuxer) NextPacketTo(p *ts.Packet) (err error) {
	return dmx.nextPacket(p)
}

// EOF is ts.ErrNoMorePackets; the unfinished unit tails are emitted before it, lowest PID first.
func (dmx *Demuxer) Next() (ev Event, err error) {
	if dmx.pending != nil {
		if !dmx.claimed {
			dmx.pending.Close()
		}
		dmx.pending = nil
		dmx.claimed = false
	}

	for {
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
		var w []byte
		if dmx.packetBuffer == nil {
			if err = dmx.ensurePacketBuffer(); err != nil {
				return 0, err
			}
		}
		if w, err = dmx.packetBuffer.Window(); err == nil && w != nil {
			var n int
			if units, n, err = dmx.walk(w, dmx.unitsArr[:0]); err != nil {
				for _, u := range units {
					poolOfPayload.put(u.buf)
				}
				dmx.pendingFatal = wrapReadError(err)
				continue
			}
			// n == 0 leaves w empty, so an unparsable first packet falls to the per-packet path below.
			w = w[:n]
		}
		if err != nil {
			err = wrapReadError(err)
		} else if len(w) == 0 {
			if err = dmx.nextPacket(&dmx.pkt); err == nil {
				units = dmx.acc.add(&dmx.pkt, dmx.unitsArr[:0])
			}
		}
		if err != nil {
			if isCancel(err) {
				return 0, err
			}
			if !errors.Is(err, ts.ErrNoMorePackets) {
				werr := fmt.Errorf("astits: fetching next packet failed: %w", err)
				if len(dmx.pendingErrs) > 0 {
					dmx.pendingFatal = werr
					continue
				}
				return 0, werr
			}
			// ErrNoMorePackets is not terminal: the reader is retried on the next call — it may grow.
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
		}

		for _, u := range units {
			d, perr := dmx.processUnit(u)
			if perr != nil {
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

// Valid until the next Next call. Tables without a syntax header (TDT/TOT/RST/ST/DIT) leave Syntax.Header zero; after a non-table event the last table event's section is still here.
func (dmx *Demuxer) Section() (pid uint16, s *psi.Section) {
	return dmx.cur.pid, dmx.cur.sec
}

// The packets the last table event's unit came from.
func (dmx *Demuxer) SectionSpan() PacketSpan {
	return dmx.cur.span
}

// Always true unless WithPSIRepeats is set, which also emits byte-identical repeats (then false).
func (dmx *Demuxer) TableChanged() bool {
	return dmx.cur.changed
}

// In effect only (current_next_indicator set); nil until one is seen.
func (dmx *Demuxer) PAT() *psi.PAT {
	return dmx.pat
}

// The last PMT of any program, current_next_indicator set only; nil until one is seen. On a multi-program stream tell them apart by Section().
func (dmx *Demuxer) PMT() *psi.PMT {
	return dmx.pmt
}

// ts.ErrNoMorePackets ends the sequence without being yielded; a recoverable error is yielded and iteration continues.
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

// Mandatory when a stream is abandoned early, and the end of the demuxer's life: do not read after it. Units still accumulating are dropped without an event, so drain them with Next first.
func (dmx *Demuxer) Close() {
	if dmx.pending != nil && !dmx.claimed {
		dmx.pending.Close()
	}
	dmx.pending = nil
	dmx.acc.close()
	if dmx.packetBuffer != nil {
		_ = dmx.packetBuffer.Close()
	}
}

// The table state survives, the emission dedup does not: tables are re-emitted on the second pass.
func (dmx *Demuxer) Rewind() (n int64, err error) {
	if dmx.packetBuffer != nil {
		if err = dmx.packetBuffer.Close(); err != nil {
			return -1, err
		}
	}
	dmx.Close()
	dmx.packetBuffer = nil
	dmx.cur = tableEvent{}
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
	// A source that seeked back leaves the wrapper holding bytes from the old position; one that could not seek must keep them, or the packets read ahead are lost.
	if br, ok := dmx.readAhead.(*bufio.Reader); ok && n >= 0 && dmx.readAhead != dmx.r {
		br.Reset(dmx.r)
	}
	return
}
