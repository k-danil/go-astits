package astits

import (
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"sync"
)

var poolOfPacketSlices = sync.Pool{
	New: func() interface{} {
		// Prepare slice of somewhat sensible initial size to minimise calls to runtime.growslice
		ps := make([]*Packet, 0, 64)
		return &ps
	},
}

// packetAccumulator keeps track of packets for a single PID and decides when to flush them
type packetAccumulator struct {
	pid        uint16
	programMap *programMap
	q          []*Packet
}

// newPacketAccumulator creates a new packet queue for a single PID
func newPacketAccumulator(pid uint16, programMap *programMap) *packetAccumulator {
	return &packetAccumulator{
		pid:        pid,
		programMap: programMap,
	}
}

// add adds a new packet for this PID to the queue
func (b *packetAccumulator) add(p *Packet) (ps []*Packet) {
	mps := b.q

	// Empty buffer if we detect a discontinuity
	if hasDiscontinuity(mps, p) {
		// Reset current slice or get new from pool and reset it
		if cap(mps) > 0 {
			mps = mps[:0]
		} else {
			mps = (*(poolOfPacketSlices.Get().(*[]*Packet)))[:0]
		}
	}

	// Throw away packet if it's the same as the previous one
	if isSameAsPrevious(mps, p) {
		return
	}

	// Flush buffer if new payload starts here
	if p.Header.PayloadUnitStartIndicator {
		ps = mps
		// Get new slice from pool and reset it
		mps = (*(poolOfPacketSlices.Get().(*[]*Packet)))[:0]
	}

	mps = append(mps, p)

	// Check if PSI payload is complete
	if b.programMap != nil &&
		(b.pid == PIDPAT || b.programMap.exists(b.pid)) &&
		isPSIComplete(mps) {
		ps = mps
		mps = nil
	}

	b.q = mps
	return
}

// packetPool represents a queue of packets for each PID in the stream
type packetPool struct {
	b map[uint32]*packetAccumulator // Indexed by PID

	programMap *programMap
}

// newPacketPool creates a new packet pool with an optional parser and programMap
func newPacketPool(programMap *programMap) *packetPool {
	return &packetPool{
		b: make(map[uint32]*packetAccumulator),

		programMap: programMap,
	}
}

// add adds a new packet to the pool
func (b *packetPool) add(p *Packet) (ps []*Packet) {
	// Throw away packet if error indicator
	if p.Header.TransportErrorIndicator {
		return
	}

	// Throw away packets that don't have a payload until we figure out what we're going to do with them
	// TODO figure out what we're going to do with them :D
	if !p.Header.HasPayload {
		return
	}

	// Make sure accumulator exists
	acc, ok := b.b[uint32(p.Header.PID)]
	if !ok {
		acc = newPacketAccumulator(p.Header.PID, b.programMap)
		b.b[uint32(p.Header.PID)] = acc
	}

	// Add to the accumulator
	return acc.add(p)
}

// dump dumps the packet pool by looking for the first item with packets inside
func (b *packetPool) dump() (ps []*Packet) {
	keys := maps.Keys(b.b)
	slices.Sort(keys)
	for _, k := range keys {
		ps = b.b[k].q
		delete(b.b, k)
		if len(ps) > 0 {
			return
		}
	}
	return
}

// hasDiscontinuity checks whether a packet is discontinuous with a set of packets
func hasDiscontinuity(ps []*Packet, p *Packet) bool {
	l := len(ps)
	return (p.Header.HasAdaptationField && p.AdaptationField.DiscontinuityIndicator) || (l > 0 && ((p.Header.HasPayload && p.Header.ContinuityCounter != (ps[l-1].Header.ContinuityCounter+1)%16) ||
		(!p.Header.HasPayload && p.Header.ContinuityCounter != ps[l-1].Header.ContinuityCounter)))
}

// isSameAsPrevious checks whether a packet is the same as the last packet of a set of packets
func isSameAsPrevious(ps []*Packet, p *Packet) bool {
	l := len(ps)
	return l > 0 && p.Header.HasPayload && p.Header.ContinuityCounter == ps[l-1].Header.ContinuityCounter
}
