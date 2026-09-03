package ts

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/k-danil/go-astits/v3/tsio"
)

// pending is the last window's bytes, discarded lazily at the next refill.
type packetBatch struct {
	bs      []byte
	len     int
	off     int
	peeker  tsio.Peeker
	tagger  tsio.Tagger
	tag     uint64
	window  int
	pending int
}

func newPeekBatch(peeker tsio.Peeker, window int) *packetBatch {
	b := &packetBatch{peeker: peeker, window: window}
	b.tagger, _ = peeker.(tsio.Tagger)
	return b
}

func (b *packetBatch) empty() bool {
	return b.off >= b.len
}

func (b *packetBatch) refill(packetSize int) (tail int, err error) {
	if b.pending > 0 {
		if _, err = b.peeker.Discard(b.pending); err != nil {
			return 0, fmt.Errorf("astits: discarding %d bytes failed: %w", b.pending, err)
		}
		b.pending = 0
	}
	want := b.window
	if have := b.peeker.Buffered(); have < want {
		want = max(have, packetSize)
	}
	// One window = one cancellation poll; a slice reader would otherwise hand over its whole remainder.
	want = min(want, cancelPollPackets*packetSize)
	var bs []byte
	if bs, err = peekUpTo(b.peeker, want); err != nil {
		return 0, fmt.Errorf("astits: peeking %d bytes failed: %w", want, err)
	}
	n := len(bs) - len(bs)%packetSize
	if n < packetSize {
		return len(bs), ErrNoMorePackets
	}
	if b.tagger != nil {
		b.tag = b.tagger.Tag()
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

// Called with the header parsed and the adaptation field not yet parsed; true drops the packet.
type PacketSkipper func(p *Packet) (skip bool)

// PacketSize 0 autodetects; SyncLock enables arbitrary-offset start alignment and mid-stream resync.
//
// SkipErrLimit and ResyncLimit share one scale: 0 tolerates nothing, -1 is unbounded, N allows N in a row. A repaired sync byte is not a damage event.
type PacketBufferConfig struct {
	PacketSize   uint
	SkipErrLimit int
	Skipper      PacketSkipper
	KeepPIDs     *PIDSet // inline PID allow-list; nil = keep all
	// Non-zero switches Next to views into the read window; the value also sizes the bufio wrapped around a reader that is not a Peeker.
	ZeroCopyBatch uint
	SyncLock      bool
	ResyncLimit   int
	// Called for each recovered damage event; nil disables reporting.
	OnRecover func(RecoverableError)
}

type PacketBuffer struct {
	packetSize    uint
	prefixLen     int
	s             PacketSkipper
	keepPIDs      *PIDSet
	r             io.Reader
	peeker        tsio.Peeker // non-nil ⇒ sync-lock mode
	tagger        tsio.Tagger
	tag           uint64
	pos           int64
	batch         *packetBatch
	syncWindowLen int
	zeroCopy      bool
	damageStreak  int
	tailAt        int64
	skipErrLimit  int
	resyncLimit   int
	onRecover     func(RecoverableError)

	// The read loops and the resync scan can run unboundedly on a live source (an absent PID skips forever), so cancellation has to live inside them.
	ctx        context.Context
	done       <-chan struct{}
	cancelPoll uint
}

// Must be a power of two: the poll check is a mask, not a modulo.
const cancelPollPackets = 1024

// No stall reported yet; offset 0 is a valid one.
const noTail = -1

func (pb *PacketBuffer) shouldPollCancel() bool {
	if pb.done == nil {
		return false
	}
	poll := pb.cancelPoll
	pb.cancelPoll++
	return poll&(cancelPollPackets-1) == 0
}

// Kept out of line so the select does not sit in the read loops.
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
		tailAt:       noTail,
	}
	if cfg.SyncLock {
		if err = pb.initSyncLock(cfg); err != nil {
			return nil, err
		}
		return
	}

	peeker := readAhead(r, cfg.ZeroCopyBatch, cfg.PacketSize)
	pb.r = peeker
	if pb.packetSize == 0 {
		if pb.packetSize, err = autoDetectPacketSize(peeker); err != nil {
			err = fmt.Errorf("astits: auto detecting packet size failed: %w", err)
			return
		}
	}
	pb.batch = newPeekBatch(peeker, peeker.Size())
	return
}

const DefaultBatchPackets = 64

func ReadAhead(r io.Reader, batchPackets uint) io.Reader {
	return ReadAheadSize(r, batchPackets, 0)
}

// packetSize 0 means the format is not known yet, so the buffer is sized for the largest one.
func ReadAheadSize(r io.Reader, batchPackets, packetSize uint) io.Reader {
	return readAhead(r, batchPackets, packetSize)
}

type peekReader interface {
	io.Reader
	tsio.Peeker
}

func readAhead(r io.Reader, batchPackets, packetSize uint) (p peekReader) {
	var ok bool
	if p, ok = r.(peekReader); ok && p.Size() >= RSPacketSize {
		return
	}
	if batchPackets == 0 {
		batchPackets = DefaultBatchPackets
	}
	if packetSize == 0 {
		packetSize = RSPacketSize
	}
	return bufio.NewReaderSize(r, max(int(batchPackets*packetSize), autoDetectWindow))
}

// Discards the consumed part of the current window (otherwise dropped lazily at the next refill), so a reader that outlives the buffer continues where the packets ended.
func (pb *PacketBuffer) Close() (err error) {
	if pb.batch == nil || pb.batch.pending == 0 {
		return
	}
	if _, err = pb.batch.peeker.Discard(pb.batch.off); err != nil {
		err = fmt.Errorf("astits: discarding %d bytes failed: %w", pb.batch.off, err)
	}
	pb.batch.pending, pb.batch.off, pb.batch.len = 0, 0, 0
	return
}

// TR 101 290 §5.2.1: sync is re-acquired on five consecutive correct sync bytes.
const resyncSyncs = 5

// A re-lock at any shift within one packet needs prefix + resyncSyncs periods in view: 5×204 = 1020. 1024 covers all three formats and lets a 1 KiB Peeker pass.
const syncScanWindow = 1024

var syncCandidates = [...]struct{ sync, size int }{
	{0, PacketSize},
	{0, RSPacketSize},
	{M2TSPacketSize - PacketSize, M2TSPacketSize},
}

func (pb *PacketBuffer) initSyncLock(cfg PacketBufferConfig) (err error) {
	bufSize := syncScanWindow
	if b := int(cfg.ZeroCopyBatch) * RSPacketSize; b > bufSize {
		bufSize = b
	}
	pb.peeker = asPeeker(pb.r, bufSize)
	pb.tagger, _ = pb.peeker.(tsio.Tagger)
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

func asPeeker(r io.Reader, bufSize int) tsio.Peeker {
	if p, ok := r.(tsio.Peeker); ok {
		return p
	}
	return bufio.NewReaderSize(r, bufSize)
}

// A short read is not an error here; the caller checks the returned length.
func peekUpTo(p tsio.Peeker, n int) (bs []byte, err error) {
	bs, err = p.Peek(n)
	if errors.Is(err, io.EOF) || errors.Is(err, bufio.ErrBufferFull) {
		err = nil
	}
	return
}

// fixedSize 0 tries every candidate size.
func scanUnit(buf []byte, fixedSize uint) (size uint, offset int, ok bool) {
	syncs := autoDetectSyncs
	// An explicit size already fixes the grid, so an input holding a single period has no recurrence left to confirm.
	if fixedSize != 0 && len(buf) < 2*int(fixedSize) {
		syncs = 1
	}
	for k := 0; k+PacketSize <= len(buf); k++ {
		for _, c := range syncCandidates {
			if fixedSize != 0 && uint(c.size) != fixedSize {
				continue
			}
			if syncLocked(buf, k+c.sync, c.size, syncs) {
				return uint(c.size), k, true
			}
		}
	}
	return
}

// Three (two recurrences) keeps a 204 stream's parity 0x47 at offset 188 from masquerading as an aligned 188 stream.
const autoDetectSyncs = 3

const autoDetectWindow = (autoDetectSyncs-1)*RSPacketSize + 1

// Requires the stream to start at a unit boundary; arbitrary-offset search is sync lock's job.
func autoDetectPacketSize(p tsio.Peeker) (packetSize uint, err error) {
	var bs []byte
	if bs, err = peekUpTo(p, autoDetectWindow); err != nil {
		err = fmt.Errorf("astits: reading first %d bytes failed: %w", autoDetectWindow, err)
		return
	}

	for _, c := range syncCandidates {
		if syncLocked(bs, c.sync, c.size, autoDetectSyncs) {
			packetSize = uint(c.size)
			return
		}
	}
	if !hasLeadingSync(bs) {
		err = ErrPacketMustStartWithASyncByte
		return
	}
	err = fmt.Errorf("astits: could not detect packet size in first %d bytes: %w", len(bs), ErrInvalidData)
	return
}

// Every sync position that fits in bs must match, with at least one recurrence unless a single period was asked for.
func syncLocked(bs []byte, start, size, syncs int) bool {
	seen := 0
	for i, off := 0, start; i < syncs && off < len(bs); i, off = i+1, off+size {
		if bs[off] != syncByte {
			return false
		}
		seen++
	}
	return seen >= min(syncs, 2)
}

// Strict form: all resyncSyncs periods must lie inside bs, so a short island never locks.
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

// Separates "no sync at a boundary" from "sync but no periodic lock" so the two report distinct errors.
func hasLeadingSync(bs []byte) bool {
	m := M2TSPacketSize - PacketSize
	return (len(bs) > 0 && bs[0] == syncByte) || (len(bs) > m && bs[m] == syncByte)
}

// n = -1 when r cannot seek.
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
		if pb.batch.empty() {
			var tail int
			if tail, err = pb.batch.refill(ps); err != nil {
				pb.dropTail(tail)
				return err
			}
		}
		bs := pb.batch.next(ps)
		if !pb.zeroCopy {
			copy(p.bs[:ps], bs)
			bs = p.bs[:ps]
		}

		p.Offset = pb.pos
		pb.pos += int64(ps)
		p.raw = bs

		var skip bool
		if skip, err = p.parse(bs, pb.s, pb.keepPIDs); err != nil {
			if errors.Is(err, ErrReservedAdaptationFieldControl) {
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
			p.Tag = pb.batch.tag
			return nil
		}
	}
}

func exhausted(counter, limit int) bool {
	return limit >= 0 && counter >= limit
}

// w is valid until the next Window or Advance: parse in place, then consume with Advance. A nil w with a nil error means fewer than one whole packet is available and nothing was consumed — fall back to Next.
func (pb *PacketBuffer) Window() (w []byte, err error) {
	ps := int(pb.packetSize)
	if pb.peeker != nil {
		w, err = pb.syncWindow(ps)
	} else {
		if pb.batch.empty() {
			var tail int
			if tail, err = pb.batch.refill(ps); err != nil {
				pb.dropTail(tail)
				return nil, err
			}
		}
		w = pb.batch.bs[pb.batch.off:pb.batch.len]
	}
	if err != nil || w == nil || pb.done == nil {
		return
	}
	// Advance adds the window's packets to the counter, so poll once it has crossed a cadence boundary.
	if pb.cancelPoll&(cancelPollPackets-1) == 0 || pb.cancelPoll >= cancelPollPackets {
		pb.cancelPoll &= cancelPollPackets - 1
		if err = pb.pollCancel(); err != nil {
			return nil, err
		}
	}
	return
}

// A corrupt sync byte is not looked for here: parse rejects it, the walk stops, and Next repairs or resyncs at that packet.
func (pb *PacketBuffer) syncWindow(ps int) (w []byte, err error) {
	want := pb.peeker.Size()
	if have := pb.peeker.Buffered(); have < want {
		want = max(have, ps)
	}
	want = min(want, cancelPollPackets*ps)
	var buf []byte
	if buf, err = peekUpTo(pb.peeker, want); err != nil {
		return nil, fmt.Errorf("astits: reading %d bytes failed: %w", want, err)
	}
	n := len(buf) - len(buf)%ps
	if n < ps {
		pb.syncWindowLen = 0
		return nil, nil
	}
	if pb.tagger != nil {
		pb.tag = pb.tagger.Tag()
	}
	pb.syncWindowLen = n
	return buf[:n], nil
}

func (pb *PacketBuffer) Tag() uint64 {
	if pb.peeker != nil {
		return pb.tag
	}
	return pb.batch.tag
}

// n must be a whole number of packets of the buffer's size and at most what remains of the window Window last returned; anything else is ErrInvalidData and consumes nothing.
func (pb *PacketBuffer) Advance(n int) (err error) {
	ps := int(pb.packetSize)
	window := pb.windowRemaining()
	if n < 0 || n%ps != 0 || n > window {
		return fmt.Errorf("astits: advancing %d bytes is not a whole count of %d-byte packets within the %d-byte window: %w", n, ps, window, ErrInvalidData)
	}
	discarded := n
	if pb.peeker != nil {
		if discarded, err = pb.peeker.Discard(n); err != nil {
			return fmt.Errorf("astits: discarding %d bytes failed: %w", n, err)
		}
		pb.syncWindowLen -= discarded
	} else {
		pb.batch.off += discarded
	}
	pb.pos += int64(discarded)
	pb.cancelPoll += uint(discarded / ps)
	return
}

func (pb *PacketBuffer) windowRemaining() int {
	if pb.peeker != nil {
		return min(pb.syncWindowLen, pb.peeker.Buffered())
	}
	return pb.batch.len - pb.batch.off
}

func (pb *PacketBuffer) Pos() int64 {
	return pb.pos
}

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
			pb.dropTail(len(buf))
			return ErrNoMorePackets
		}

		if pb.tagger != nil {
			pb.tag = pb.tagger.Tag()
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
			if errors.Is(err, ErrReservedAdaptationFieldControl) {
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
			p.Tag = pb.tag
			return nil
		}
	}
}

// TR 101 290 §5.2.1: with the next period's sync intact this is a sync_byte_error, not a loss. The repaired byte goes into p.bs — never into the caller's Peeker buffer.
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

func (pb *PacketBuffer) discard(n int) (err error) {
	if _, err = pb.peeker.Discard(n); err != nil {
		return fmt.Errorf("astits: discarding %d bytes failed: %w", n, err)
	}
	pb.pos += int64(n)
	return
}

// Outside the damage budget, like a filtered packet.
func (pb *PacketBuffer) dropReserved(p *Packet, ps int) {
	if pb.onRecover != nil {
		pb.onRecover(RecoverableError{Kind: ErrorKindPacketDrop, PID: p.Header.PID, Offset: p.Offset, Dropped: int64(ps), Err: ErrReservedAdaptationFieldControl})
	}
}

// The tail is left in the reader so a source that is still growing can complete the packet; pos therefore stays put and the stall is reported once.
func (pb *PacketBuffer) dropTail(n int) {
	if n == 0 || pb.tailAt == pb.pos {
		return
	}
	pb.tailAt = pb.pos
	if pb.onRecover != nil {
		pb.onRecover(RecoverableError{Kind: ErrorKindPacketDrop, PID: PIDUnset, Offset: pb.pos, Dropped: int64(n), Err: ErrShortPacket})
	}
}

// Each slide keeps the last resyncSyncs-1 periods behind so a boundary they straddle is rescanned; the -1 is the byte already scanned. A window short of syncScanWindow is end of input.
func (pb *PacketBuffer) resync(ps int) (err error) {
	for windows := 0; ; windows++ {
		if err = pb.pollCancel(); err != nil {
			return
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
