package astits

// pidSlot — состояние одного PID: аккумулятор пакетов + счётчик статистики.
type pidSlot struct {
	q     *PacketList
	pid   uint16
	stats uint32
}

// add accumulates packets of one PID and decides when to flush them
func (s *pidSlot) add(p *Packet, pm *programMap) (pl *PacketList) {
	mps := s.q

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
	if pm != nil &&
		(p.Header.PID == PIDPAT || pm.existsUnlocked(p.Header.PID)) &&
		isPSIComplete(mps) {
		pl = mps
		mps = nil
	}

	s.q = mps
	return
}

// packetPool represents a queue of packets for each PID in the stream.
// Компактный список активных PID: их единицы, линейный скан дешевле и map-хэша
// на каждый пакет, и зануления плоского [8192]-массива на каждый инстанс
// (демуксер живёт один чанк — фиксированный футпринт умножается на RPS).
type packetPool struct {
	slots      []pidSlot
	programMap *programMap
}

const packetPoolPreallocPIDs = 8

// newPacketPool creates a new packet pool with an optional parser and programMap
func newPacketPool(programMap *programMap) *packetPool {
	return &packetPool{
		slots:      make([]pidSlot, 0, packetPoolPreallocPIDs),
		programMap: programMap,
	}
}

// slot возвращает слот PID'а; указатель нельзя держать через append (реаллокация)
func (b *packetPool) slot(pid uint16) *pidSlot {
	for i := range b.slots {
		if b.slots[i].pid == pid {
			return &b.slots[i]
		}
	}
	b.slots = append(b.slots, pidSlot{pid: pid})
	return &b.slots[len(b.slots)-1]
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

	slot := b.slot(p.Header.PID)
	slot.stats++
	return slot.add(p, b.programMap)
}

// dumpUnlocked dumps the packet pool by looking for the first item with packets
// inside. PID'ы отдаются по возрастанию — порядок хвостовых данных на EOF
// совпадает с исходной map-реализацией с сортировкой ключей.
func (b *packetPool) dumpUnlocked() (pl *PacketList) {
	min := -1
	for i := range b.slots {
		if b.slots[i].q.IsEmpty() {
			continue
		}
		if min < 0 || b.slots[i].pid < b.slots[min].pid {
			min = i
		}
	}
	if min < 0 {
		return
	}
	pl = b.slots[min].q
	b.slots[min].q = nil
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
