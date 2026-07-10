package ts

import (
	"bufio"
	"fmt"
	"io"
)

// packetBatch is the zero-copy read buffer: packets are returned as views into bs,
// valid until the next refill.
type packetBatch struct {
	bs  []byte
	len int
	off int
}

func newPacketBatch(packetSize, batchPackets uint) *packetBatch {
	return &packetBatch{bs: make([]byte, packetSize*batchPackets)}
}

func (b *packetBatch) empty() bool {
	return b.off >= b.len
}

// refill drops a trailing partial packet: the stream either ends there or is torn,
// exactly like the per-packet ReadFull path treats it.
func (b *packetBatch) refill(r io.Reader, packetSize int) (err error) {
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

func (b *packetBatch) next(packetSize int) (bs []byte) {
	bs = b.bs[b.off : b.off+packetSize]
	b.off += packetSize
	return
}

// PacketSkipper represents an object capable of skipping a packet before parsing its payload. Its header and adaptation field is parsed and provided to the object.
// Use this option if you need to filter out unwanted packets from your pipeline. NextPacket() will return the next unskipped packet if any.
type PacketSkipper func(p *Packet) (skip bool)

var EmptySkipper = func(_ *Packet) (skip bool) { return }

// PacketBuffer represents a packet buffer
type PacketBuffer struct {
	packetSize     uint
	s              PacketSkipper
	r              io.Reader
	pos            int64
	batch          *packetBatch // nil = copy mode
	skipErrCounter uint
	skipErrLimit   uint
}

// NewPacketBuffer creates a new packet buffer
func NewPacketBuffer(r io.Reader, packetSize, skipErrLimit uint, s PacketSkipper, zeroCopyBatch uint) (pb *PacketBuffer, err error) {
	pb = &PacketBuffer{
		packetSize:   packetSize,
		s:            s,
		r:            r,
		skipErrLimit: skipErrLimit,
	}

	if pb.packetSize == 0 {
		if pb.packetSize, err = autoDetectPacketSize(r); err != nil {
			err = fmt.Errorf("astits: auto detecting packet size failed: %w", err)
			return
		}
	}

	if zeroCopyBatch > 0 {
		pb.batch = newPacketBatch(pb.packetSize, zeroCopyBatch)
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

	for _, c := range [...]struct{ start, size int }{
		{0, PacketSize},
		{0, RSPacketSize},
		{M2TSPacketSize - PacketSize, M2TSPacketSize},
	} {
		if syncLocked(bs, c.start, c.size) {
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
// one recurrence. Checking all in-buffer periods keeps a 204 stream's parity
// 0x47 at offset 188 from locking as 188 on a full window, while a short
// two-packet stream still locks on its single recurrence.
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
		if err == io.EOF || err == bufio.ErrBufferFull {
			err = nil
		}
		if err != nil {
			return
		}
		return copy(b, bs), false, nil
	}

	n, err = io.ReadFull(r, b)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
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

// nextView fetches the next packet as a view into the batch buffer: no per-packet
// read and no copy, the view dies on the refill triggered by a later call.
func (pb *PacketBuffer) nextView(p *Packet) (err error) {
	ps := int(pb.packetSize)
	for {
		if pb.batch.empty() {
			if err = pb.batch.refill(pb.r, ps); err != nil {
				return err
			}
		}

		// parse overwrites Header and nils AF/Payload itself — no per-packet Reset needed
		p.Offset = pb.pos
		pb.pos += int64(ps)

		view := pb.batch.next(ps)
		p.raw = view

		var skip bool
		if skip, err = p.parse(view, pb.s); err != nil {
			if skip && pb.skipErrCounter < pb.skipErrLimit {
				pb.skipErrCounter++
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

func (pb *PacketBuffer) PacketSize() uint {
	return pb.packetSize
}

// Next fetches the next packet from the buffer
func (pb *PacketBuffer) Next(p *Packet) (err error) {
	if pb.batch != nil {
		return pb.nextView(p)
	}

	bs := p.bs[:pb.packetSize]
	p.raw = bs

	var skip bool
	// Loop to make sure we return a packet even if first packets are skipped
	for {
		if _, err = io.ReadFull(pb.r, bs); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				err = ErrNoMorePackets
			} else {
				err = fmt.Errorf("astits: reading %d bytes failed: %w", pb.packetSize, err)
			}
			return err
		}

		p.Offset = pb.pos
		pb.pos += int64(pb.packetSize)
		if skip, err = p.parse(bs, pb.s); err != nil {
			if skip && pb.skipErrCounter < pb.skipErrLimit {
				pb.skipErrCounter++
			} else {
				return fmt.Errorf("astits: building packet failed: %w", err)
			}
		} else {
			pb.skipErrCounter = 0
		}

		if !skip {
			break
		}
	}

	return
}
