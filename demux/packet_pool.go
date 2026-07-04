package demux

import (
	"github.com/k-danil/go-astits/v2/internal/pidmap"
	"github.com/k-danil/go-astits/v2/ts"
)

// pidSlot is the per-PID state: a packet accumulator plus a stats counter.
type pidSlot struct {
	q     *ts.PacketList
	stats uint32
}

// add accumulates packets of one PID and decides when to flush them
func (s *pidSlot) add(p *ts.Packet, b *packetPool) (pl *ts.PacketList) {
	mps := s.q

	if !mps.IsEmpty() {
		if isSameAsPrevious(mps.Tail(), p) {
			b.recyclePacket(p)
			return
		}
		if hasDiscontinuity(mps.Tail(), p) {
			b.recycleListPackets(mps)
		}
		if p.Header.PayloadUnitStartIndicator {
			pl = mps
			mps = ts.NewPacketList()
		}
	} else {
		mps = ts.NewPacketList()
	}

	mps.PushBack(p)

	// Check if PSI payload is complete
	if b.programMap != nil &&
		(p.Header.PID == ts.PIDPAT || b.programMap.Has(p.Header.PID)) &&
		isPSIComplete(mps) {
		pl = mps
		mps = nil
	}

	s.q = mps
	return
}

// packetPool represents a queue of packets for each PID in the stream
type packetPool struct {
	slots      pidmap.Map[pidSlot]
	free       ts.PacketFreeList
	programMap *pidmap.Map[uint16]

	keysArr [packetPoolPreallocPIDs]uint16
	valsArr [packetPoolPreallocPIDs]pidSlot
}

const packetPoolPreallocPIDs = 8

// init readies the pool in place: slot slices start on the embedded arrays so a
// short-lived demuxer does not pay heap allocations for them.
func (b *packetPool) init(programMap *pidmap.Map[uint16]) {
	b.slots = pidmap.Map[pidSlot]{Keys: b.keysArr[:0], Vals: b.valsArr[:0]}
	b.free = ts.PacketFreeList{}
	b.programMap = programMap
}

func (b *packetPool) getPacket() (p *ts.Packet) {
	return b.free.Get()
}

func (b *packetPool) recyclePacket(p *ts.Packet) {
	b.free.Put(p)
}

// recycleListPackets links the whole packet chain onto the freelist in one splice;
// the list itself stays with the caller
func (b *packetPool) recycleListPackets(pl *ts.PacketList) {
	b.free.Splice(pl)
}

// recycle is for after parseData: payloads are already copied, nothing views p.bs
func (b *packetPool) recycle(pl *ts.PacketList) {
	b.free.Splice(pl)
	pl.Close()
}

// close returns everything held to the pools: unfinished slot lists and the freelist
func (b *packetPool) close() {
	for i := range b.slots.Vals {
		if q := b.slots.Vals[i].q; q != nil {
			b.recycle(q)
			b.slots.Vals[i].q = nil
		}
	}
	b.drain()
}

func (b *packetPool) drain() {
	b.free.Drain()
}

// addUnlocked adds a new packet to the pool
func (b *packetPool) addUnlocked(p *ts.Packet) (pl *ts.PacketList) {
	// Throw away packet if error indicator
	if p.Header.TransportErrorIndicator {
		b.recyclePacket(p)
		return
	}

	// Throw away packets that don't have a payload until we figure out what we're going to do with them
	// TODO figure out what we're going to do with them :D
	if !p.Header.HasPayload {
		b.recyclePacket(p)
		return
	}

	slot := b.slots.GetOrAdd(p.Header.PID)
	slot.stats++
	return slot.add(p, b)
}

// dumpUnlocked dumps the packet pool by looking for the first item with packets
// inside. PIDs are yielded in ascending order so that the EOF tail data order
// matches the original map implementation with sorted keys.
func (b *packetPool) dumpUnlocked() (pl *ts.PacketList) {
	minIdx := -1
	for i := range b.slots.Vals {
		if b.slots.Vals[i].q.IsEmpty() {
			continue
		}
		if minIdx < 0 || b.slots.Keys[i] < b.slots.Keys[minIdx] {
			minIdx = i
		}
	}
	if minIdx < 0 {
		return
	}
	pl = b.slots.Vals[minIdx].q
	b.slots.Vals[minIdx].q = nil
	return
}

// hasDiscontinuity checks whether a packet is discontinuous with a set of packets
func hasDiscontinuity(pl *ts.Packet, p *ts.Packet) bool {
	return (p.Header.HasAdaptationField && p.AdaptationField.DiscontinuityIndicator) || ((p.Header.HasPayload && p.Header.ContinuityCounter != (pl.Header.ContinuityCounter+1)%16) ||
		(!p.Header.HasPayload && p.Header.ContinuityCounter != pl.Header.ContinuityCounter))

}

// isSameAsPrevious checks whether a packet is the same as the last packet of a set of packets
func isSameAsPrevious(pl *ts.Packet, p *ts.Packet) bool {
	return p.Header.HasPayload && p.Header.ContinuityCounter == pl.Header.ContinuityCounter
}
