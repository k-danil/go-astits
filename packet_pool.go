package astits

// pidSlot — состояние одного PID: аккумулятор пакетов + счётчик статистики.
type pidSlot struct {
	q     *PacketList
	stats uint32
}

// add accumulates packets of one PID and decides when to flush them
func (s *pidSlot) add(p *Packet, b *packetPool) (pl *PacketList) {
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
			mps = NewPacketList()
		}
	} else {
		mps = NewPacketList()
	}

	mps.PushBack(p)

	// Check if PSI payload is complete
	if b.programMap != nil &&
		(p.Header.PID == PIDPAT || b.programMap.existsUnlocked(p.Header.PID)) &&
		isPSIComplete(mps) {
		pl = mps
		mps = nil
	}

	s.q = mps
	return
}

// packetPool represents a queue of packets for each PID in the stream
type packetPool struct {
	slots      pidMap[pidSlot]
	free       *Packet // фрилист демуксера: пакеты не покидают nextData-цикл, sync.Pool не нужен
	programMap *programMap
}

const packetPoolPreallocPIDs = 8

// newPacketPool creates a new packet pool with an optional parser and programMap
func newPacketPool(programMap *programMap) *packetPool {
	return &packetPool{
		slots:      newPidMap[pidSlot](packetPoolPreallocPIDs),
		programMap: programMap,
	}
}

func (b *packetPool) getPacket() (p *Packet) {
	if p = b.free; p != nil {
		b.free = p.next
		p.Reset()
		return
	}
	return NewPacket()
}

func (b *packetPool) recyclePacket(p *Packet) {
	p.next = b.free
	b.free = p
}

// recycleListPackets пришивает цепочку списка к фрилисту одним сцеплением;
// сам список остаётся у вызывающего
func (b *packetPool) recycleListPackets(pl *PacketList) {
	if pl.head != nil {
		pl.tail.next = b.free
		b.free = pl.head
	}
	*pl = PacketList{}
}

// recycle — после parseData: payload'ы уже скопированы, на p.bs никто не смотрит
func (b *packetPool) recycle(pl *PacketList) {
	b.recycleListPackets(pl)
	poolOfPacketList.Put(pl)
}

// close возвращает пулам всё удерживаемое: недособранные списки слотов и фрилист
func (b *packetPool) close() {
	for i := range b.slots.vals {
		if q := b.slots.vals[i].q; q != nil {
			b.recycle(q)
			b.slots.vals[i].q = nil
		}
	}
	b.drain()
}

// drain сливает фрилист в глобальный пул — на EOF, чтобы короткоживущий
// демуксер вернул пакеты, а не отдал их GC
func (b *packetPool) drain() {
	for p := b.free; p != nil; {
		next := p.next
		p.Close()
		p = next
	}
	b.free = nil
}

// addUnlocked adds a new packet to the pool
func (b *packetPool) addUnlocked(p *Packet) (pl *PacketList) {
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

	slot := b.slots.getOrAdd(p.Header.PID)
	slot.stats++
	return slot.add(p, b)
}

// dumpUnlocked dumps the packet pool by looking for the first item with packets
// inside. PID'ы отдаются по возрастанию — порядок хвостовых данных на EOF
// совпадает с исходной map-реализацией с сортировкой ключей.
func (b *packetPool) dumpUnlocked() (pl *PacketList) {
	minIdx := -1
	for i := range b.slots.vals {
		if b.slots.vals[i].q.IsEmpty() {
			continue
		}
		if minIdx < 0 || b.slots.keys[i] < b.slots.keys[minIdx] {
			minIdx = i
		}
	}
	if minIdx < 0 {
		return
	}
	pl = b.slots.vals[minIdx].q
	b.slots.vals[minIdx].q = nil
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
