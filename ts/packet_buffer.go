package ts

import (
	"bufio"
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

	// When peeker is set, bs is a view into the reader's own buffer rather than
	// an owned copy: a bufio-backed source already holds the bytes, so there is
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

// refill drops a trailing partial packet: the stream either ends there or is torn,
// exactly like the per-packet ReadFull path treats it.
func (b *packetBatch) refill(r io.Reader, packetSize int) (err error) {
	if b.peeker != nil {
		return b.refillPeek(packetSize)
	}
	var n int
	if n, err = io.ReadFull(r, b.bs); n < packetSize {
		if err == io.EOF || err == io.ErrUnexpectedEOF || err == nil {
			return ErrNoMorePackets
		}
		return fmt.Errorf("astits: reading %d bytes failed: %w", len(b.bs), err)
	}
	b.len = n - n%packetSize
	b.off = 0
	return nil
}

// refillPeek views the next whole-packet window straight out of the reader's
// buffer, no copy. The previous window is dropped first; a trailing partial
// packet is left buffered for the next peek.
func (b *packetBatch) refillPeek(packetSize int) (err error) {
	if b.pending > 0 {
		if _, err = b.peeker.Discard(b.pending); err != nil {
			return fmt.Errorf("astits: discarding %d bytes failed: %w", b.pending, err)
		}
		b.pending = 0
	}
	var bs []byte
	if bs, err = peekUpTo(b.peeker, b.window); err != nil {
		return fmt.Errorf("astits: peeking %d bytes failed: %w", b.window, err)
	}
	n := len(bs) - len(bs)%packetSize
	if n < packetSize {
		return ErrNoMorePackets
	}
	b.bs = bs
	b.len = n
	b.off = 0
	b.pending = n
	return nil
}

func (b *packetBatch) next(packetSize int) (bs []byte) {
	bs = b.bs[b.off : b.off+packetSize]
	b.off += packetSize
	return
}

// PacketSkipper represents an object capable of skipping a packet before parsing its payload. Its header and adaptation field is parsed and provided to the object.
// Use this option if you need to filter out unwanted packets from your pipeline. NextPacket() will return the next unskipped packet if any.
type PacketSkipper func(p *Packet) (skip bool)

var EmptySkipper = func(_ *Packet) (skip bool) { return }

// Peeker is a reader that can look ahead without consuming and drop bytes it has
// looked at; *bufio.Reader satisfies it. A reader that provides its own (e.g. a
// UDP datagram reassembler) is used directly under sync lock; any other reader
// is wrapped in bufio.
//
// The contract, matching *bufio.Reader: Peek returns up to n bytes without
// consuming (fewer, with a non-nil error such as io.EOF, only at end of input),
// and must accept n up to a few hundred bytes (a boundary scan window); Discard
// drops exactly n bytes, where n never exceeds what a preceding Peek returned.
type Peeker interface {
	Peek(n int) ([]byte, error)
	Discard(n int) (discarded int, err error)
}

// PacketBufferConfig configures NewPacketBuffer. PacketSize 0 autodetects.
// SyncLock enables arbitrary-offset start alignment and mid-stream resync via
// Peek; ResyncLimit 0 resyncs indefinitely.
type PacketBufferConfig struct {
	PacketSize    uint
	SkipErrLimit  uint
	Skipper       PacketSkipper
	ZeroCopyBatch uint
	SyncLock      bool
	ResyncLimit   uint
	// OnRecover, when set, is called for each recovered damage event (sync loss,
	// dropped packet); nil keeps the silent fast path. Only invoked on the cold
	// error branches, never on a clean read.
	OnRecover func(RecoverableError)
}

// PacketBuffer represents a packet buffer
type PacketBuffer struct {
	packetSize     uint
	prefixLen      int // M2TS TP_extra_header ahead of the sync byte; 0 otherwise
	s              PacketSkipper
	r              io.Reader
	peeker         Peeker // non-nil ⇒ sync-lock mode
	pos            int64
	batch          *packetBatch // nil = copy mode
	zeroCopy       bool
	skipErrCounter uint
	skipErrLimit   uint
	resyncCounter  uint
	resyncLimit    uint // 0 = unlimited
	onRecover      func(RecoverableError)
}

// NewPacketBuffer creates a new packet buffer
func NewPacketBuffer(r io.Reader, cfg PacketBufferConfig) (pb *PacketBuffer, err error) {
	pb = &PacketBuffer{
		packetSize:   cfg.PacketSize,
		s:            cfg.Skipper,
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
			if _, buffered := r.(*bufio.Reader); !buffered {
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

// newBatch picks the view buffer. A *bufio.Reader already holds the stream in its
// own buffer, so peek views straight into it rather than copying it into a second
// batch of our own; any other reader gets an owned batch it is read (copied) into.
func (pb *PacketBuffer) newBatch(batchPackets uint) *packetBatch {
	if br, ok := pb.r.(*bufio.Reader); ok && br.Size() >= int(pb.packetSize) {
		return newPeekBatch(br, br.Size())
	}
	return newPacketBatch(pb.packetSize, batchPackets)
}

// syncScanWindow is how many bytes a boundary search peeks: room to slide the
// unit offset across one widest packet and still confirm the sync period.
const syncScanWindow = RSPacketSize + (autoDetectSyncs-1)*RSPacketSize + (M2TSPacketSize - PacketSize)

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

	var buf []byte
	if buf, err = peekUpTo(pb.peeker, syncScanWindow); err != nil {
		return fmt.Errorf("astits: sync lock peek failed: %w", err)
	}
	size, off, ok := scanUnit(buf, cfg.PacketSize)
	if !ok {
		return fmt.Errorf("astits: could not lock onto a sync byte in first %d bytes: %w", len(buf), ErrInvalidData)
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
			if syncLocked(buf, k+c.sync, c.size) {
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
		if syncLocked(bs, c.sync, c.size) {
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
// fits in bs (up to autoDetectSyncs of them) holds a sync byte, with at least
// one recurrence. Requiring all in-window periods to match (not just a run of
// two) keeps a 204 stream's parity 0x47 at offset 188 from locking as 188 while
// the window still has room for the next check; a short two-packet stream still
// locks on its single recurrence.
func syncLocked(bs []byte, start, size int) bool {
	seen := 0
	for i, off := 0, start; i < autoDetectSyncs && off < len(bs); i, off = i+1, off+size {
		if bs[off] != syncByte {
			return false
		}
		seen++
	}
	return seen >= 2
}

// hasLeadingSync separates "no sync at a unit boundary" from "sync present but
// no periodic lock", so the two failures report distinct errors.
func hasLeadingSync(bs []byte) bool {
	m := M2TSPacketSize - PacketSize
	return (len(bs) > 0 && bs[0] == syncByte) || (len(bs) > m && bs[m] == syncByte)
}

// peek fills b from r and reports how many bytes it got. A *bufio.Reader is
// peeked (not consumed, so shouldRewind is false); any other reader is read and
// must be rewound or synced past the consumed bytes afterwards. A short stream
// is not an error here — the caller decides whether it held enough to detect.
func peek(r io.Reader, b []byte) (n int, shouldRewind bool, err error) {
	if br, ok := r.(*bufio.Reader); ok {
		var bs []byte
		bs, err = br.Peek(len(b))
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
		var bs []byte
		if pb.batch != nil {
			if pb.batch.empty() {
				if err = pb.batch.refill(pb.r, ps); err != nil {
					return err
				}
			}
			bs = pb.batch.next(ps)
		} else {
			bs = p.bs[:ps]
			if _, err = io.ReadFull(pb.r, bs); err != nil {
				if err == io.EOF || errors.Is(err, io.ErrUnexpectedEOF) {
					return ErrNoMorePackets
				}
				return fmt.Errorf("astits: reading %d bytes failed: %w", ps, err)
			}
		}

		p.Offset = pb.pos
		pb.pos += int64(ps)
		p.raw = bs

		var skip bool
		if skip, err = p.parse(bs, pb.s); err != nil {
			if skip && pb.skipErrCounter < pb.skipErrLimit {
				pb.skipErrCounter++
				if pb.onRecover != nil {
					pb.onRecover(RecoverableError{Kind: ErrorKindPacketDrop, PID: PIDUnset, Offset: p.Offset, Err: err})
				}
			} else {
				return fmt.Errorf("astits: building packet failed: %w", err)
			}
		} else {
			pb.skipErrCounter = 0
		}
		if !skip {
			return nil
		}
	}
}

// nextSync fetches the next packet under sync lock: it peeks a packet, resyncs
// on a missing sync byte, then copies it out (or hands back the peeked view in
// zero-copy mode) and drops it from the buffer. An aligned but unparseable
// packet is dropped as a damage event, not a fatal error, so one corrupt packet
// on a lossy feed does not kill the stream.
func (pb *PacketBuffer) nextSync(p *Packet) (err error) {
	ps := int(pb.packetSize)
	for {
		var buf []byte
		if buf, err = peekUpTo(pb.peeker, ps); err != nil {
			return fmt.Errorf("astits: reading %d bytes failed: %w", ps, err)
		}
		if len(buf) < ps {
			return ErrNoMorePackets
		}

		if buf[pb.prefixLen] != syncByte {
			if pb.onRecover != nil {
				pb.onRecover(RecoverableError{Kind: ErrorKindSyncLoss, PID: PIDUnset, Offset: pb.pos, Err: ErrPacketMustStartWithASyncByte})
			}
			if err = pb.resync(ps); err != nil {
				return err
			}
			continue
		}

		pkt := buf[:ps]
		if !pb.zeroCopy {
			copy(p.bs[:ps], pkt)
			pkt = p.bs[:ps]
		}
		p.raw = pkt

		p.Offset = pb.pos
		var skip bool
		if skip, err = p.parse(pkt, pb.s); err != nil {
			// Sync was present, so scanning won't help: drop the damaged packet.
			if pb.onRecover != nil {
				pb.onRecover(RecoverableError{Kind: ErrorKindPacketDrop, PID: PIDUnset, Offset: p.Offset, Err: err})
			}
			if err = pb.dropDamaged(ps); err != nil {
				return err
			}
			continue
		}
		pb.resyncCounter = 0

		if _, err = pb.peeker.Discard(ps); err != nil {
			return fmt.Errorf("astits: discarding %d bytes failed: %w", ps, err)
		}
		pb.pos += int64(ps)

		if !skip {
			return nil
		}
	}
}

// dropDamaged discards one packet after a damage event and enforces ResyncLimit.
func (pb *PacketBuffer) dropDamaged(ps int) (err error) {
	if pb.noteRecovery() {
		return fmt.Errorf("astits: sync recovery exhausted after %d events: %w", pb.resyncCounter, ErrInvalidData)
	}
	if _, err = pb.peeker.Discard(ps); err != nil {
		return fmt.Errorf("astits: discarding %d bytes failed: %w", ps, err)
	}
	pb.pos += int64(ps)
	return
}

// noteRecovery records one damage event and reports whether ResyncLimit is hit.
// The counter is cleared only by a cleanly parsed packet, so it measures a run
// of consecutive damage (corrupt packets and fruitless scan windows alike).
func (pb *PacketBuffer) noteRecovery() bool {
	pb.resyncCounter++
	return pb.resyncLimit > 0 && pb.resyncCounter >= pb.resyncLimit
}

// resync scans forward for the next unit boundary after a lost sync byte and
// discards up to it. It keeps a straddling tail across windows so a boundary
// that needs lookahead still locks; ResyncLimit caps the fruitless windows.
func (pb *PacketBuffer) resync(ps int) (err error) {
	for {
		var buf []byte
		if buf, err = peekUpTo(pb.peeker, syncScanWindow); err != nil {
			return fmt.Errorf("astits: resync peek failed: %w", err)
		}
		if len(buf) < ps {
			return ErrNoMorePackets
		}

		if k := pb.scanResync(buf, ps); k >= 0 {
			if _, err = pb.peeker.Discard(k); err != nil {
				return fmt.Errorf("astits: resync discard failed: %w", err)
			}
			pb.pos += int64(k)
			return nil
		}

		drop := max(len(buf)-(autoDetectSyncs-1)*ps, 1)
		if _, err = pb.peeker.Discard(drop); err != nil {
			return fmt.Errorf("astits: resync discard failed: %w", err)
		}
		pb.pos += int64(drop)

		if pb.noteRecovery() {
			return fmt.Errorf("astits: resync exhausted after %d events: %w", pb.resyncCounter, ErrInvalidData)
		}
	}
}

// scanResync returns the offset (≥1) of the next unit boundary in buf, or -1.
// Scanning small offsets first, where the full window confirms the period,
// finds a strong lock before any weak one a payload 0x47 could form near the end.
func (pb *PacketBuffer) scanResync(buf []byte, ps int) int {
	for k := 1; k+ps <= len(buf); k++ {
		if syncLocked(buf, k+pb.prefixLen, ps) {
			return k
		}
	}
	return -1
}
