package astits

type PacketList struct {
	head *Packet
	tail *Packet
	next *Packet
	size int
}

func MakePacketList(p *Packet) *PacketList {
	return &PacketList{
		head: p,
		tail: p,
		size: len(p.Payload),
	}
}

func (pl *PacketList) Add(p *Packet) {
	pl.tail.next = p
	p.prev = pl.tail
	pl.size += len(p.Payload)
	pl.tail = p
}

func (pl *PacketList) GetNext() *Packet {
	ret := pl.next
	pl.next = pl.next.next
	return ret
}

func (pl *PacketList) GetSize() int {
	return pl.size
}

func (pl *PacketList) Clear() {
	if pl.tail != nil {
		for p := pl.tail; p != nil; {
			cur := p
			p = p.prev
			*cur = Packet{Payload: cur.Payload[:0]}
			PoolOfPacket.Put(cur)
		}
	}
}
