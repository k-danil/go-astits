package astits

type PacketList struct {
	head, tail *Packet

	l, s int
}

func NewPacketList() *PacketList {
	return &PacketList{}
}

func (pl *PacketList) PushBack(p *Packet) {
	if pl.head != nil {
		pl.tail.next = p
	} else {
		pl.head = p
	}

	pl.s += len(p.Payload)
	pl.l++

	pl.tail = p
}

func (pl *PacketList) Tail() *Packet {
	return pl.tail
}

func (pl *PacketList) Head() *Packet {
	return pl.head
}

func (pl *PacketList) IsEmpty() bool {
	return pl == nil || pl.head == nil
}

func (pl *PacketList) Size() int {
	return pl.s
}

func (pl *PacketList) Length() int {
	return pl.l
}

func (pl *PacketList) Clear() {
	head := pl.head
	for head != nil {
		cur := head
		head = cur.next
		cur.Close()
	}
	*pl = PacketList{}
}
