package ts

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

// PacketFreeList is a demuxer-local freelist: packets never leave the demuxing
// loop, no sync.Pool round-trips.
type PacketFreeList struct {
	head *Packet
}

func (fl *PacketFreeList) Get() (p *Packet) {
	if p = fl.head; p != nil {
		fl.head = p.next
		p.Reset()
		return
	}
	return NewPacket()
}

func (fl *PacketFreeList) Put(p *Packet) {
	p.next = fl.head
	fl.head = p
}

// Splice links the whole packet chain onto the freelist in one splice;
// the emptied list stays with the caller.
func (fl *PacketFreeList) Splice(pl *PacketList) {
	if pl.head != nil {
		pl.tail.next = fl.head
		fl.head = pl.head
	}
	*pl = PacketList{}
}

// Drain flushes the freelist into the global pool — on EOF, so that a short-lived
// demuxer returns its packets instead of feeding them to the GC.
func (fl *PacketFreeList) Drain() {
	for p := fl.head; p != nil; {
		next := p.next
		p.Close()
		p = next
	}
	fl.head = nil
}
