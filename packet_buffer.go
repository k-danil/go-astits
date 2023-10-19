package astits

import (
	"bufio"
	"fmt"
	"io"

	"github.com/asticode/go-astikit"
)

// packetBuffer represents a packet buffer
type packetBuffer struct {
	packetSize     uint
	s              PacketSkipper
	r              io.Reader
	skipErrCounter uint
	skipErrLimit   uint
}

// newPacketBuffer creates a new packet buffer
func newPacketBuffer(r io.Reader, packetSize, skipErrLimit uint, s PacketSkipper) (pb *packetBuffer, err error) {
	// Init
	pb = &packetBuffer{
		packetSize:   packetSize,
		s:            s,
		r:            r,
		skipErrLimit: skipErrLimit,
	}

	// Packet size is not set
	if pb.packetSize == 0 {
		// Auto detect packet size
		if pb.packetSize, err = autoDetectPacketSize(r); err != nil {
			err = fmt.Errorf("astits: auto detecting packet size failed: %w", err)
			return
		}
	}
	return
}

// autoDetectPacketSize updates the packet size based on the first bytes
// Minimum packet size is 188 and is bounded by 2 sync bytes
// Assumption is made that the first byte of the reader is a sync byte
func autoDetectPacketSize(r io.Reader) (packetSize uint, err error) {
	// Read first bytes
	const l = 193
	var bs = make([]byte, l)
	shouldRewind, rerr := peek(r, bs)
	if rerr != nil {
		err = fmt.Errorf("astits: reading first %d bytes failed: %w", l, rerr)
		return
	}

	// Packet must start with a sync byte
	if bs[0] != syncByte {
		err = ErrPacketMustStartWithASyncByte
		return
	}

	// Look for sync bytes
	for idx, b := range bs {
		if b == syncByte && idx >= MpegTsPacketSize {
			// Update packet size
			packetSize = uint(idx)

			if !shouldRewind {
				return
			}

			// Rewind or sync reader
			var n int64
			if n, err = rewind(r); err != nil {
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
	err = fmt.Errorf("astits: only one sync byte detected in first %d bytes", l)
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

// rewind rewinds the reader if possible, otherwise n = -1
func rewind(r io.Reader) (n int64, err error) {
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

// next fetches the next packet from the buffer
func (pb *packetBuffer) next(p *Packet) (err error) {
	p.s = pb.packetSize
	bs := p.bs[:pb.packetSize]
	bi := astikit.NewBytesIterator(bs)

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

		// Parse packet
		if skip, err = p.parse(bi, pb.s); err != nil {
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

		bi.Seek(0)
		p.Reset()
	}

	return
}
