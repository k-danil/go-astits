package astits

import (
	"sort"
	"sync"
)

// packetBuffer represents a buffer of packets
type packetBuffer struct {
	b map[uint16][]*Packet // Indexed by PID
	m *sync.Mutex
}

// newPacketBuffer creates a new packet buffer
func newPacketBuffer() *packetBuffer {
	return &packetBuffer{
		b: make(map[uint16][]*Packet),
		m: &sync.Mutex{},
	}
}

// add adds a new packet to the buffer
func (b *packetBuffer) add(p *Packet) (ps []*Packet) {
	// Lock
	b.m.Lock()
	defer b.m.Unlock()

	// Init buffer or empty buffer if discontinuity
	var mps []*Packet
	var ok bool
	if mps, ok = b.b[p.Header.PID]; !ok || hasDiscontinuity(mps, p) {
		mps = []*Packet{}
	}

	// Add packet
	if len(mps) > 0 || (len(mps) == 0 && p.Header.PayloadUnitStartIndicator) {
		mps = append(mps, p)
	}

	// Check payload unit start indicator
	if p.Header.PayloadUnitStartIndicator && len(mps) > 1 {
		ps = mps[:len(mps)-1]
		mps = []*Packet{p}
	}

	// Assign
	b.b[p.Header.PID] = mps
	return
}

// dump dumps the packet buffer by looking for the first item with packets inside
func (b *packetBuffer) dump() (ps []*Packet) {
	b.m.Lock()
	defer b.m.Unlock()
	var keys []int
	for k := range b.b {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	for _, k := range keys {
		ps = b.b[uint16(k)]
		delete(b.b, uint16(k))
		if len(ps) > 0 {
			return
		}
	}
	return
}

// hasDiscontinuity checks whether a packet is discontinuous with a set of packets
func hasDiscontinuity(ps []*Packet, p *Packet) bool {
	return (p.Header.HasAdaptationField && p.AdaptationField.DiscontinuityIndicator) ||
		(len(ps) > 0 && p.Header.ContinuityCounter != (ps[len(ps)-1].Header.ContinuityCounter+1)%16)
}
