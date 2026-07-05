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

// autoDetectPacketSize updates the packet size based on the first bytes
// Minimum packet size is 188 and is bounded by 2 sync bytes
// Assumption is made that the first byte of the reader is a sync byte
func autoDetectPacketSize(r io.Reader) (packetSize uint, err error) {
	const l = 193
	var bs = make([]byte, l)
	shouldRewind, rerr := peek(r, bs)
	if rerr != nil {
		err = fmt.Errorf("astits: reading first %d bytes failed: %w", l, rerr)
		return
	}

	if bs[0] != syncByte {
		err = ErrPacketMustStartWithASyncByte
		return
	}

	for idx, b := range bs {
		if b == syncByte && idx >= PacketSize {
			packetSize = uint(idx)

			if !shouldRewind {
				return
			}

			var n int64
			if n, err = Rewind(r); err != nil {
				err = fmt.Errorf("astits: rewinding failed: %w", err)
				return
			} else if n == -1 {
				var ls = packetSize - (l - packetSize)
				if _, err = r.Read(make([]byte, ls)); err != nil {
					err = fmt.Errorf("astits: reading %d bytes to sync reader failed: %w", ls, err)
					return
				}
			}
			return
		}
	}
	err = fmt.Errorf("astits: only one sync byte detected in first %d bytes: %w", l, ErrInvalidData)
	return
}

// bufio.Reader can't be rewinded, which leads to packet loss on packet size autodetection
// but it has handy Peek() method
// so what we do here is peeking bytes for bufio.Reader and falling back to rewinding/syncing for all other readers
func peek(r io.Reader, b []byte) (shouldRewind bool, err error) {
	if br, ok := r.(*bufio.Reader); ok {
		var bs []byte
		bs, err = br.Peek(len(b))
		if err != nil {
			return
		}
		copy(b, bs)
		return false, nil
	}

	_, err = r.Read(b)
	shouldRewind = true
	return
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
