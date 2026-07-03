package astits

import "sync"

type PacketList struct {
	head, tail *Packet

	l, s int
}

var poolOfPacketList = sync.Pool{
	New: func() interface{} {
		return &PacketList{}
	},
}

func NewPacketList() *PacketList {
	pl, _ := poolOfPacketList.Get().(*PacketList)
	return pl
}

// Close clears the list and returns it to the pool: only when no references remain
func (pl *PacketList) Close() {
	pl.Clear()
	poolOfPacketList.Put(pl)
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
