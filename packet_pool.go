package astits

import (
	"sort"
)

// packetAccumulator keeps track of packets for a single PID and decides when to flush them
type packetAccumulator struct {
	pid        uint16
	programMap *programMap
	q          *PacketList
}

// newPacketAccumulator creates a new packet queue for a single PID
func newPacketAccumulator(pid uint16, programMap *programMap) *packetAccumulator {
	return &packetAccumulator{
		pid:        pid,
		programMap: programMap,
	}
}

// add adds a new packet for this PID to the queue
func (b *packetAccumulator) add(p *Packet) (pl *PacketList) {
	mps := b.q

	if !mps.IsEmpty() {
		if isSameAsPrevious(mps.Tail(), p) {
			return
		}
		if hasDiscontinuity(mps.Tail(), p) {
			mps.Clear()
		}
		if p.Header.PayloadUnitStartIndicator {
			pl = mps
			mps = NewPacketList()
		}
	} else {
		mps = NewPacketList()
	}

	mps.PushBack(p)

	// Check if PSI payload is complete
	if b.programMap != nil &&
		(b.pid == PIDPAT || b.programMap.existsUnlocked(b.pid)) &&
		isPSIComplete(mps) {
		pl = mps
		mps = nil
	}

	b.q = mps
	return
}

// packetPool represents a queue of packets for each PID in the stream
type packetPool struct {
	b map[uint64]*packetAccumulator // Indexed by PID

	programMap *programMap
}

// newPacketPool creates a new packet pool with an optional parser and programMap
func newPacketPool(programMap *programMap) *packetPool {
	return &packetPool{
		b: make(map[uint64]*packetAccumulator),

		programMap: programMap,
	}
}

// addUnlocked adds a new packet to the pool
func (b *packetPool) addUnlocked(p *Packet) (pl *PacketList) {
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
	acc, ok := b.b[uint64(p.Header.PID)]
	if !ok {
		acc = newPacketAccumulator(p.Header.PID, b.programMap)
		b.b[uint64(p.Header.PID)] = acc
	}

	// Add to the accumulator
	return acc.add(p)
}

// dumpUnlocked dumps the packet pool by looking for the first item with packets inside
func (b *packetPool) dumpUnlocked() (pl *PacketList) {
	var keys []int
	for k := range b.b {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	for _, k := range keys {
		pl = b.b[uint64(k)].q
		delete(b.b, uint64(k))
		if !pl.IsEmpty() {
			return
		}
	}
	return
}

// hasDiscontinuity checks whether a packet is discontinuous with a set of packets
func hasDiscontinuity(pl *Packet, p *Packet) bool {
	return (p.Header.HasAdaptationField && p.AdaptationField.DiscontinuityIndicator) || ((p.Header.HasPayload && p.Header.ContinuityCounter != (pl.Header.ContinuityCounter+1)%16) ||
		(!p.Header.HasPayload && p.Header.ContinuityCounter != pl.Header.ContinuityCounter))

}

// isSameAsPrevious checks whether a packet is the same as the last packet of a set of packets
func isSameAsPrevious(pl *Packet, p *Packet) bool {
	return p.Header.HasPayload && p.Header.ContinuityCounter == pl.Header.ContinuityCounter
}
