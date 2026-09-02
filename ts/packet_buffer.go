package ts

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
)

// packetBatch is the zero-copy read buffer: packets are returned as views into bs,
// valid until the next refill.
type packetBatch struct {
	bs  []byte
	len int
	off int
	// partial is the trailing part-packet of the last read, kept behind the
	// window and moved ahead of the next read so the reader stays aligned.
	partial int

	// When peeker is set, bs is a view into the reader's own buffer rather than
	// an owned copy: a Peeker-backed source already holds the bytes, so there is
	// no reason to copy them into a second batch. pending is the previous
	// window's consumed bytes, dropped before the next peek.
	peeker  Peeker
	window  int
	pending int
}

func newPacketBatch(packetSize, batchPackets uint) *packetBatch {
	return &packetBatch{bs: make([]byte, packetSize*batchPackets)}
}

func newPeekBatch(peeker Peeker, window int) *packetBatch {
	return &packetBatch{peeker: peeker, window: window}
}

func (b *packetBatch) empty() bool {
	return b.off >= b.len
}

// refill reads the next window. At end of input the bytes short of a whole
// packet are consumed and returned as tail with ErrNoMorePackets, so the loss
// is reported once.
func (b *packetBatch) refill(r io.Reader, packetSize int) (tail int, err error) {
	if b.peeker != nil {
		return b.refillPeek(packetSize)
	}
	carry := b.partial
	copy(b.bs, b.bs[b.len:b.len+carry])
	b.partial = 0
	var n int
	n, err = io.ReadFull(r, b.bs[carry:])
	n += carry
	if n < packetSize {
		if err == io.EOF || err == io.ErrUnexpectedEOF || err == nil {
			return n, ErrNoMorePackets
		}
		return 0, fmt.Errorf("astits: reading %d bytes failed: %w", len(b.bs), err)
	}
	b.partial = n % packetSize
	b.len = n - b.partial
	b.off = 0
	return 0, nil
}

// refillPeek views the next whole-packet window straight out of the reader's
// buffer, no copy. The previous window is dropped first; a trailing partial
// packet is left buffered for the next peek.
func (b *packetBatch) refillPeek(packetSize int) (tail int, err error) {
	if b.pending > 0 {
		if _, err = b.peeker.Discard(b.pending); err != nil {
			return 0, fmt.Errorf("astits: discarding %d bytes failed: %w", b.pending, err)
		}
		b.pending = 0
	}
	var bs []byte
	if bs, err = peekUpTo(b.peeker, b.window); err != nil {
		return 0, fmt.Errorf("astits: peeking %d bytes failed: %w", b.window, err)
	}
	n := len(bs) - len(bs)%packetSize
	if n < packetSize {
		if _, err = b.peeker.Discard(len(bs)); err != nil {
			return 0, fmt.Errorf("astits: discarding %d bytes failed: %w", len(bs), err)
		}
		return len(bs), ErrNoMorePackets
	}
	b.bs = bs
	b.len = n
	b.off = 0
	b.pending = n
	return 0, nil
}

func (b *packetBatch) next(packetSize int) (bs []byte) {
	bs = b.bs[b.off : b.off+packetSize]
	b.off += packetSize
	return
}

// PacketSkipper represents an object capable of skipping a packet before parsing its payload. Its header and adaptation field is parsed and provided to the object.
// Use this option if you need to filter out unwanted packets from your pipeline. NextPacket() will return the next unskipped packet if any.
type PacketSkipper func(p *Packet) (skip bool)

// Peeker is a reader that can look ahead without consuming and drop bytes it has
// looked at; *bufio.Reader satisfies it. A reader that provides its own (e.g. a
// UDP datagram reassembler) is used directly; any other reader is wrapped in
// bufio.
//
// The contract, matching *bufio.Reader: Peek returns up to n bytes without
// consuming (fewer, with a non-nil error such as io.EOF, only at end of input),
// and must accept n up to Size(); Discard drops exactly n bytes, where n never
// exceeds what a preceding Peek returned; Size reports that peek ceiling, which
// must cover one boundary-scan window (syncScanWindow, 1024 bytes): a smaller
// one fails NewPacketBuffer under SyncLock.
type Peeker interface {
	Peek(n int) ([]byte, error)
	Discard(n int) (discarded int, err error)
	Size() int
}

// PacketBufferConfig configures NewPacketBuffer. PacketSize 0 autodetects.
// SyncLock enables arbitrary-offset start alignment and mid-stream resync via
// Peek.
//
// SkipErrLimit and ResyncLimit share one scale: 0 (the default) tolerates
// nothing, -1 is unbounded, N allows N in a row. SkipErrLimit bounds the streak
// of consecutive damage events — a packet that fails to parse at an aligned
// position (dropped, in either mode) and, under sync lock, a lost sync, counted
// once when the loss is detected whatever the resync's outcome — reset by a
// clean packet; a repaired sync byte costs nothing. ResyncLimit is the number
// of scan windows one resync may spend re-locking (sync lock only): with the
// default a lost sync is fatal at once.
type PacketBufferConfig struct {
	PacketSize    uint
	SkipErrLimit  int
	Skipper       PacketSkipper
	KeepPIDs      *PIDSet // inline PID allow-list; nil = keep all
	ZeroCopyBatch uint
	SyncLock      bool
	ResyncLimit   int
	// OnRecover, when set, is called for each recovered damage event (sync loss,
	// repaired sync byte, dropped packet); nil keeps the silent fast path. Only
	// invoked on the cold error branches, never on a clean read.
	OnRecover func(RecoverableError)
}

// PacketBuffer represents a packet buffer
type PacketBuffer struct {
	packetSize   uint
	prefixLen    int // M2TS TP_extra_header ahead of the sync byte; 0 otherwise
	s            PacketSkipper
	keepPIDs     *PIDSet
	r            io.Reader
	peeker       Peeker // non-nil ⇒ sync-lock mode
	pos          int64
	batch        *packetBatch // nil = copy mode
	zeroCopy     bool
	damageStreak int
	skipErrLimit int
	resyncLimit  int
	onRecover    func(RecoverableError)

	// Cancellation lives here rather than in the caller: the read loops and the
	// resync scan can all run unboundedly on a live source (an absent PID skips
	// forever, an unlimited resync scans forever), and a check outside Next
	// cannot reach any of them.
	ctx        context.Context
	done       <-chan struct{}
	cancelPoll uint
}

// cancelPollPackets must be a power of two: the check is a mask, not a modulo.
// A non-blocking select is a call into runtime.selectnbrecv — far too costly to
// run per packet — so it is checked on the first pass and every 1024th after.
const cancelPollPackets = 1024

// shouldPollCancel keeps the per-packet path free of calls: it is inlined into
// the read loops, and only the rare true result reaches the select below.
func (pb *PacketBuffer) shouldPollCancel() bool {
	if pb.done == nil {
		return false
	}
	poll := pb.cancelPoll
	pb.cancelPoll++
	return poll&(cancelPollPackets-1) == 0
}

// The pragma keeps the select out of the read loops: inlined, it would sit in
// the loop body on every packet to run once per cancelPollPackets.
//
//go:noinline
func (pb *PacketBuffer) pollCancel() error {
	select {
	case <-pb.done:
		return pb.ctx.Err()
	default:
		return nil
	}
}

// NewPacketBuffer creates a new packet buffer. ctx cancels the read loops; pass
// context.Background() for a buffer that runs to the end of the reader.
func NewPacketBuffer(ctx context.Context, r io.Reader, cfg PacketBufferConfig) (pb *PacketBuffer, err error) {
	pb = &PacketBuffer{
		ctx:          ctx,
		done:         ctx.Done(),
		packetSize:   cfg.PacketSize,
		s:            cfg.Skipper,
		keepPIDs:     cfg.KeepPIDs,
		r:            r,
		zeroCopy:     cfg.ZeroCopyBatch > 0,
		skipErrLimit: cfg.SkipErrLimit,
		resyncLimit:  cfg.ResyncLimit,
		onRecover:    cfg.OnRecover,
	}
	if cfg.SyncLock {
		if err = pb.initSyncLock(cfg); err != nil {
			return nil, err
		}
		return
	}

	if pb.packetSize == 0 {
		// A non-seekable, non-buffered reader can't be rewound after peeking, so
		// autodetect would consume (and drop) the packets it inspects and skew
		// Packet.Offset. Buffer it so the peek costs nothing.
		if _, seekable := r.(io.Seeker); !seekable {
			if _, peekable := r.(Peeker); !peekable {
				pb.r = bufio.NewReader(r)
			}
		}
		if pb.packetSize, err = autoDetectPacketSize(pb.r); err != nil {
			err = fmt.Errorf("astits: auto detecting packet size failed: %w", err)
			return
		}
	}

	if cfg.ZeroCopyBatch > 0 {
		pb.batch = pb.newBatch(cfg.ZeroCopyBatch)
	}
	return
}

// newBatch picks the view buffer. A Peeker already holds the stream in its own
// buffer, so peek views straight into it rather than copying it into a second
// batch of our own; any other reader gets an owned batch it is read (copied) into.
func (pb *PacketBuffer) newBatch(batchPackets uint) *packetBatch {
	if p, ok := pb.r.(Peeker); ok && p.Size() >= int(pb.packetSize) {
		return newPeekBatch(p, p.Size())
	}
	return newPacketBatch(pb.packetSize, batchPackets)
}

// resyncSyncs is how many periodic sync bytes a re-lock after a sync loss
// needs: TR 101 290 §5.2.1 acquires sync on five consecutive correct ones (and
// declares it lost on two corrupt ones), so a shorter island inside damage
// stays part of the loss. The start-of-stream lock keeps autoDetectSyncs.
const resyncSyncs = 5

// syncScanWindow is how many bytes a boundary search peeks. A re-lock at any
// shift within one packet needs that shift plus resyncSyncs periods in view,
// i.e. more than prefix + resyncSyncs×size: 5×204 = 1020 for Reed-Solomon (no
// prefix), 4 + 5×192 = 964 for M2TS, 5×188 = 940 for plain TS. 1024 covers all
// three and lets a Peeker of exactly one kibibyte pass.
const syncScanWindow = 1024

// syncCandidates are the (sync offset within the unit, packet size) pairs the
// detector and the resync scanner recognise; the unit begins at byte 0, M2TS
// carries a 4-byte prefix so its sync sits at offset 4.
var syncCandidates = [...]struct{ sync, size int }{
	{0, PacketSize},
	{0, RSPacketSize},
	{M2TSPacketSize - PacketSize, M2TSPacketSize},
}

// initSyncLock wraps the reader in a Peeker, then scans for the first unit
// boundary and discards up to it so the first packet read is aligned.
func (pb *PacketBuffer) initSyncLock(cfg PacketBufferConfig) (err error) {
	bufSize := syncScanWindow
	if b := int(cfg.ZeroCopyBatch) * RSPacketSize; b > bufSize {
		bufSize = b
	}
	pb.peeker = asPeeker(pb.r, bufSize)
	if size := pb.peeker.Size(); size < syncScanWindow {
		return fmt.Errorf("astits: peeker size %d is below the %d-byte scan window: %w", size, syncScanWindow, ErrInvalidData)
	}

	var buf []byte
	if buf, err = peekUpTo(pb.peeker, syncScanWindow); err != nil {
		return fmt.Errorf("astits: sync lock peek failed: %w", err)
	}
	size, off, ok := scanUnit(buf, cfg.PacketSize)
	if !ok {
		return fmt.Errorf("astits: could not lock onto a sync byte in first %d bytes: %w", len(buf), ErrInvalidData)
	}
	if off > 0 && pb.onRecover != nil {
		pb.onRecover(RecoverableError{Kind: ErrorKindSyncLoss, PID: PIDUnset, Offset: 0, Dropped: int64(off), Err: ErrPacketMustStartWithASyncByte})
	}
	if _, err = pb.peeker.Discard(off); err != nil {
		return fmt.Errorf("astits: discarding %d bytes to unit boundary failed: %w", off, err)
	}
	pb.pos += int64(off)
	pb.packetSize = size
	if size == M2TSPacketSize {
		pb.prefixLen = M2TSPacketSize - PacketSize
	}
	return
}

func asPeeker(r io.Reader, bufSize int) Peeker {
	if p, ok := r.(Peeker); ok {
		return p
	}
	return bufio.NewReaderSize(r, bufSize)
}

// peekUpTo peeks up to n bytes, treating a short stream as success — the caller
// decides from the returned length whether it got enough.
func peekUpTo(p Peeker, n int) (bs []byte, err error) {
	bs, err = p.Peek(n)
	if err == io.EOF || errors.Is(err, bufio.ErrBufferFull) {
		err = nil
	}
	return
}

// scanUnit finds the first unit boundary in buf: the smallest offset with a
// periodic sync lock for a candidate size (only fixedSize when non-zero).
func scanUnit(buf []byte, fixedSize uint) (size uint, offset int, ok bool) {
	for k := 0; k+PacketSize <= len(buf); k++ {
		for _, c := range syncCandidates {
			if fixedSize != 0 && uint(c.size) != fixedSize {
				continue
			}
			if syncLocked(buf, k+c.sync, c.size, autoDetectSyncs) {
				return uint(c.size), k, true
			}
		}
	}
	return
}

// autoDetectSyncs is how many sync bytes at a candidate period must line up to
// accept its packet size. Three (two recurrences) drops a coincidental 0x47
// lock from ~1/256 to ~1/2^16 and stops a 204 stream's parity byte at offset
// 188 from masquerading as an aligned 188 stream.
const autoDetectSyncs = 3

// autoDetectWindow holds autoDetectSyncs syncs for the widest candidate (204).
const autoDetectWindow = (autoDetectSyncs-1)*RSPacketSize + 1

// autoDetectPacketSize infers the packet size by locking onto a periodic sync
// byte: 188 (TS), 192 (M2TS, a 4-byte TP_extra_header before each sync) or 204
// (TS plus a 16-byte Reed-Solomon suffix). The unit begins at byte 0 in every
// format; the stream must be aligned to a unit boundary (arbitrary-offset sync
// search is a separate concern).
func autoDetectPacketSize(r io.Reader) (packetSize uint, err error) {
	bs := make([]byte, autoDetectWindow)
	n, shouldRewind, rerr := peek(r, bs)
	if rerr != nil {
		err = fmt.Errorf("astits: reading first %d bytes failed: %w", autoDetectWindow, rerr)
		return
	}
	bs = bs[:n]

	for _, c := range syncCandidates {
		if syncLocked(bs, c.sync, c.size, autoDetectSyncs) {
			packetSize = uint(c.size)
			break
		}
	}
	if packetSize == 0 {
		if !hasLeadingSync(bs) {
			err = ErrPacketMustStartWithASyncByte
		} else {
			err = fmt.Errorf("astits: could not detect packet size in first %d bytes: %w", n, ErrInvalidData)
		}
		return
	}

	if !shouldRewind {
		return
	}
	var rn int64
	if rn, err = Rewind(r); err != nil {
		err = fmt.Errorf("astits: rewinding failed: %w", err)
		return
	} else if rn == -1 {
		// Non-seekable: peek consumed n bytes; drop the rest of the partial unit
		// so the first packet read lands on a boundary.
		if skip := (int(packetSize) - n%int(packetSize)) % int(packetSize); skip > 0 {
			if _, err = io.ReadFull(r, make([]byte, skip)); err != nil {
				err = fmt.Errorf("astits: reading %d bytes to sync reader failed: %w", skip, err)
				return
			}
		}
	}
	return
}

// syncLocked reports whether every sync position at start, start+size, … that
// fits in bs (up to syncs of them) holds a sync byte, with at least one
// recurrence. Requiring all in-window periods to match (not just a run of two)
// keeps a 204 stream's parity 0x47 at offset 188 from locking as 188 while the
// window still has room for the next check; a short two-packet stream still
// locks on its single recurrence.
func syncLocked(bs []byte, start, size, syncs int) bool {
	seen := 0
	for i, off := 0, start; i < syncs && off < len(bs); i, off = i+1, off+size {
		if bs[off] != syncByte {
			return false
		}
		seen++
	}
	return seen >= 2
}

// resyncLocked is the strict form for a re-lock: all resyncSyncs periods must
// lie inside bs and hold a sync byte, so an island shorter than that never
// locks, however close to the window's end it sits.
func resyncLocked(bs []byte, start, size int) bool {
	last := start + (resyncSyncs-1)*size
	if last >= len(bs) {
		return false
	}
	for off := start; off <= last; off += size {
		if bs[off] != syncByte {
			return false
		}
	}
	return true
}

// hasLeadingSync separates "no sync at a unit boundary" from "sync present but
// no periodic lock", so the two failures report distinct errors.
func hasLeadingSync(bs []byte) bool {
	m := M2TSPacketSize - PacketSize
	return (len(bs) > 0 && bs[0] == syncByte) || (len(bs) > m && bs[m] == syncByte)
}

// peek fills b from r and reports how many bytes it got. A Peeker is peeked (not
// consumed, so shouldRewind is false); any other reader is read and must be
// rewound or synced past the consumed bytes afterwards. A short stream is not an
// error here — the caller decides whether it held enough to detect.
func peek(r io.Reader, b []byte) (n int, shouldRewind bool, err error) {
	if p, ok := r.(Peeker); ok {
		var bs []byte
		bs, err = p.Peek(len(b))
		if err == io.EOF || errors.Is(err, bufio.ErrBufferFull) {
			err = nil
		}
		if err != nil {
			return
		}
		return copy(b, bs), false, nil
	}

	n, err = io.ReadFull(r, b)
	if err == io.EOF || errors.Is(err, io.ErrUnexpectedEOF) {
		err = nil
	}
	return n, true, err
}

// Rewind rewinds the reader if possible, otherwise n = -1
func Rewind(r io.Reader) (n int64, err error) {
	if s, ok := r.(io.Seeker); ok {
		if n, err = s.Seek(0, 0); err != nil {
			err = fmt.Errorf("astits: seeking to 0 failed: %w", err)
			return
		}
		return
	}
	n = -1
	return
}

func (pb *PacketBuffer) PacketSize() uint {
	return pb.packetSize
}

// Next fetches the next packet. In zero-copy mode the packet is a view into the
// batch buffer (valid until the next refill); otherwise it is read into the
// packet's own bytes. Skipped packets and budgeted parse errors are read past;
// sync-lock mode goes through nextSync.
func (pb *PacketBuffer) Next(p *Packet) (err error) {
	if pb.peeker != nil {
		return pb.nextSync(p)
	}

	ps := int(pb.packetSize)
	for {
		if pb.shouldPollCancel() {
			if err = pb.pollCancel(); err != nil {
				return
			}
		}
		var bs []byte
		if pb.batch != nil {
			if pb.batch.empty() {
				var tail int
				if tail, err = pb.batch.refill(pb.r, ps); err != nil {
					pb.dropTail(tail)
					return err
				}
			}
			bs = pb.batch.next(ps)
		} else {
			bs = p.bs[:ps]
			var n int
			if n, err = io.ReadFull(pb.r, bs); err != nil {
				if err == io.EOF || errors.Is(err, io.ErrUnexpectedEOF) {
					pb.dropTail(n)
					return ErrNoMorePackets
				}
				return fmt.Errorf("astits: reading %d bytes failed: %w", ps, err)
			}
		}

		p.Offset = pb.pos
		pb.pos += int64(ps)
		p.raw = bs

		var skip bool
		if skip, err = p.parse(bs, pb.s, pb.keepPIDs); err != nil {
			if err == ErrReservedAdaptationFieldControl {
				pb.dropReserved(p, ps)
				continue
			}
			if pb.onRecover != nil {
				pb.onRecover(RecoverableError{Kind: ErrorKindPacketDrop, PID: PIDUnset, Offset: p.Offset, Dropped: int64(ps), Err: err})
			}
			if exhausted(pb.damageStreak, pb.skipErrLimit) {
				return fmt.Errorf("astits: packet damage streak exhausted after %d events: %w", pb.damageStreak, err)
			}
			pb.damageStreak++
			continue
		}
		pb.damageStreak = 0
		if !skip {
			return nil
		}
	}
}

// exhausted applies the shared limit scale: 0 tolerates nothing, -1 never
// gives up, N allows N events before the next one is fatal.
func exhausted(counter, limit int) bool {
	return limit >= 0 && counter >= limit
}

// An aligned but unparseable packet is dropped as a damage event under the
// streak budget, not as a fatal error on its own.
func (pb *PacketBuffer) nextSync(p *Packet) (err error) {
	ps := int(pb.packetSize)
	for {
		if pb.shouldPollCancel() {
			if err = pb.pollCancel(); err != nil {
				return
			}
		}
		var buf []byte
		if buf, err = peekUpTo(pb.peeker, ps); err != nil {
			return fmt.Errorf("astits: reading %d bytes failed: %w", ps, err)
		}
		if len(buf) < ps {
			if _, err = pb.peeker.Discard(len(buf)); err != nil {
				return fmt.Errorf("astits: discarding %d bytes failed: %w", len(buf), err)
			}
			pb.dropTail(len(buf))
			return ErrNoMorePackets
		}

		pkt := buf[:ps]
		if buf[pb.prefixLen] != syncByte {
			var repaired bool
			if pkt, repaired, err = pb.repairSyncByte(p, ps); err != nil {
				return err
			}
			if !repaired {
				if exhausted(pb.damageStreak, pb.skipErrLimit) {
					return fmt.Errorf("astits: packet damage streak exhausted after %d events: %w", pb.damageStreak, ErrPacketMustStartWithASyncByte)
				}
				pb.damageStreak++
				lost := pb.pos
				err = pb.resync(ps)
				if pb.onRecover != nil {
					pb.onRecover(RecoverableError{Kind: ErrorKindSyncLoss, PID: PIDUnset, Offset: lost, Dropped: pb.pos - lost, Err: ErrPacketMustStartWithASyncByte})
				}
				if err != nil {
					return err
				}
				continue
			}
		} else if !pb.zeroCopy {
			copy(p.bs[:ps], pkt)
			pkt = p.bs[:ps]
		}
		p.raw = pkt

		p.Offset = pb.pos
		var skip bool
		if skip, err = p.parse(pkt, pb.s, pb.keepPIDs); err != nil {
			if err == ErrReservedAdaptationFieldControl {
				pb.dropReserved(p, ps)
				if err = pb.discard(ps); err != nil {
					return err
				}
				continue
			}
			// Sync was present, so scanning won't help: drop the damaged packet.
			if pb.onRecover != nil {
				pb.onRecover(RecoverableError{Kind: ErrorKindPacketDrop, PID: PIDUnset, Offset: p.Offset, Dropped: int64(ps), Err: err})
			}
			if err = pb.dropDamaged(ps); err != nil {
				return err
			}
			continue
		}
		pb.damageStreak = 0

		if _, err = pb.peeker.Discard(ps); err != nil {
			return fmt.Errorf("astits: discarding %d bytes failed: %w", ps, err)
		}
		pb.pos += int64(ps)

		if !skip {
			return nil
		}
	}
}

// repairSyncByte applies the TR 101 290 §5.2.1 hysteresis to a corrupt sync
// byte: with the next period's sync intact it is a sync_byte_error, not a
// loss, so the packet is read from a copy with the byte restored and delivered.
// The copy keeps the hot parse and a caller's Peeker buffer untouched.
func (pb *PacketBuffer) repairSyncByte(p *Packet, ps int) (pkt []byte, repaired bool, err error) {
	var buf []byte
	if buf, err = peekUpTo(pb.peeker, 2*ps); err != nil {
		return nil, false, fmt.Errorf("astits: reading %d bytes failed: %w", 2*ps, err)
	}
	if len(buf) < 2*ps || buf[pb.prefixLen+ps] != syncByte {
		return
	}
	pkt = p.bs[:ps]
	copy(pkt, buf[:ps])
	pkt[pb.prefixLen] = syncByte
	if pb.onRecover != nil {
		pb.onRecover(RecoverableError{Kind: ErrorKindSyncByte, PID: PIDUnset, Offset: pb.pos, Err: ErrPacketMustStartWithASyncByte})
	}
	return pkt, true, nil
}

func (pb *PacketBuffer) dropDamaged(ps int) (err error) {
	if exhausted(pb.damageStreak, pb.skipErrLimit) {
		return fmt.Errorf("astits: packet damage streak exhausted after %d events: %w", pb.damageStreak, ErrInvalidData)
	}
	pb.damageStreak++
	return pb.discard(ps)
}

// discard is the cold-path consume; the per-packet path keeps its own inline
// Discard so no call is added there.
func (pb *PacketBuffer) discard(n int) (err error) {
	if _, err = pb.peeker.Discard(n); err != nil {
		return fmt.Errorf("astits: discarding %d bytes failed: %w", n, err)
	}
	pb.pos += int64(n)
	return
}

// dropReserved reports a discarded adaptation_field_control '00' packet; it is
// outside the damage budget, like a filtered packet.
func (pb *PacketBuffer) dropReserved(p *Packet, ps int) {
	if pb.onRecover != nil {
		pb.onRecover(RecoverableError{Kind: ErrorKindPacketDrop, PID: p.Header.PID, Offset: p.Offset, Dropped: int64(ps), Err: ErrReservedAdaptationFieldControl})
	}
}

// dropTail accounts for the bytes short of a whole packet at end of input,
// already consumed by the caller.
func (pb *PacketBuffer) dropTail(n int) {
	if n == 0 {
		return
	}
	if pb.onRecover != nil {
		pb.onRecover(RecoverableError{Kind: ErrorKindPacketDrop, PID: PIDUnset, Offset: pb.pos, Dropped: int64(n), Err: ErrShortPacket})
	}
	pb.pos += int64(n)
}

// resync scans forward for the next unit boundary after a lost sync byte and
// discards up to it, ResyncLimit windows at most. Each slide keeps the last
// resyncSyncs-1 periods behind so a boundary they straddle is scanned again
// with room to confirm it; the byte at the slide itself was already scanned.
// A window short of syncScanWindow is the end of input: whatever did not lock
// there is consumed into the loss.
func (pb *PacketBuffer) resync(ps int) (err error) {
	for windows := 0; ; windows++ {
		if pb.shouldPollCancel() {
			if err = pb.pollCancel(); err != nil {
				return
			}
		}
		if exhausted(windows, pb.resyncLimit) {
			return fmt.Errorf("astits: resync exhausted after %d windows: %w", windows, ErrInvalidData)
		}
		var buf []byte
		if buf, err = peekUpTo(pb.peeker, syncScanWindow); err != nil {
			return fmt.Errorf("astits: resync peek failed: %w", err)
		}

		if k := pb.scanResync(buf, ps); k >= 0 {
			return pb.discard(k)
		}
		if len(buf) < syncScanWindow {
			if err = pb.discard(len(buf)); err != nil {
				return err
			}
			return ErrNoMorePackets
		}
		if err = pb.discard(len(buf) - pb.prefixLen - (resyncSyncs-1)*ps - 1); err != nil {
			return err
		}
	}
}

func (pb *PacketBuffer) scanResync(buf []byte, ps int) int {
	for k := 1; k+pb.prefixLen+(resyncSyncs-1)*ps < len(buf); k++ {
		if resyncLocked(buf, k+pb.prefixLen, ps) {
			return k
		}
	}
	return -1
}
