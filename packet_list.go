package astits

type PacketList struct {
	head *Packet
	tail *Packet

	it *Packet
	s  int
}

func NewPacketList() *PacketList {
	return &PacketList{}
}

func (pl *PacketList) Add(p *Packet) {
	if pl.head != nil {
		pl.tail.next = p
		p.prev = pl.tail
	} else {
		pl.head = p
		pl.it = p
	}
	pl.s += len(p.Payload)
	pl.tail = p
}

func (pl *PacketList) GetTail() *Packet {
	return pl.tail
}

func (pl *PacketList) GetHead() *Packet {
	return pl.head
}

func (pl *PacketList) IteratorGet() *Packet {
	return pl.it
}

func (pl *PacketList) IteratorNext() *Packet {
	pl.it = pl.it.next
	return pl.it
}

func (pl *PacketList) IteratorPrev() *Packet {
	pl.it = pl.it.prev
	return pl.it
}

func (pl *PacketList) IteratorReset() {
	pl.it = pl.head
}

func (pl *PacketList) IsEmpty() bool {
	return pl == nil || pl.head == nil
}

func (pl *PacketList) GetSize() int {
	return pl.s
}

func (pl *PacketList) Clear() {
	tail := pl.tail
	for tail != nil {
		cur := tail
		tail = cur.prev
		cur.Close()
	}
	*pl = PacketList{}
}
