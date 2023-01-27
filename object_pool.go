package astits

import "sync"

// poolOfPacketSlice global variable is used to ease access to pool from any place of the code
var poolOfPacketSlice = &poolPacketSlice{
	sp: sync.Pool{
		New: func() interface{} {
			// Prepare the slice of somewhat sensible initial size to minimise calls to runtime.growslice
			ps := make([]*Packet, 0, 64)
			return &ps
		},
	},
}

// poolOfTempPayload global variable is used to ease access to pool from any place of the code
var poolOfTempPayload = &poolTempPayload{
	sp: sync.Pool{
		New: func() interface{} {
			// Prepare the slice of somewhat sensible initial size to minimize calls to runtime.growslice
			d := make([]byte, 0, 1024)
			return &d
		},
	},
}

// poolOfPacket global variable is used to ease access to pool from any place of the code
var poolOfPacket = &poolPacket{}

// poolOfPESData global variable is used to ease access to pool from any place of the code
var poolOfPESData = &poolPESData{}

// poolPacketSlice is a pool of packet references slices
// you should use it whenever this kind of object created or destroyed
type poolPacketSlice struct {
	sp sync.Pool
}

// get returns the slice of packet references of a zero length and some capacity
func (pps *poolPacketSlice) get() []*Packet {
	// Reset slice length to use with append
	return (*(pps.sp.Get().(*[]*Packet)))[:0]
}

// put returns reference to packet slice back to pool
// don't use packet slice after a call to put
func (pps *poolPacketSlice) put(ps []*Packet) {
	pps.sp.Put(&ps)
}

// poolTempPayload is a pool for temporary payload in parseData()
// don't use it anywhere else to avoid pool pollution
type poolTempPayload struct {
	sp sync.Pool
}

// get returns the byte slice of a 'size' length
func (ptp *poolTempPayload) get(size int) (payload []byte) {
	payload = *(ptp.sp.Get().(*[]byte))
	// Reset slice length or grow it to requested size to use with copy
	if cap(payload) >= size {
		payload = payload[:size]
	} else {
		n := size - cap(payload)
		payload = append(payload[:cap(payload)], make([]byte, n)...)[:size]
	}
	return
}

// put returns reference to the payload slice back to pool
// don't use the payload after a call to put
func (ptp *poolTempPayload) put(payload []byte) {
	ptp.sp.Put(&payload)
}

// poolPESData represent fabric class for the Packet objects
// If sync.Pool is used and Packet received via Demuxer.NextPacket() than you should dispose it yourself
type poolPacket struct {
	sp *sync.Pool
}

// get returns empty Packet
// If sync.Pool is used than Packet may contain Payload slice, this slice will be reset to zero length
func (pp *poolPacket) get() (p *Packet) {
	if pp.sp != nil {
		if p = pp.sp.Get().(*Packet); p != nil {
			if p.Payload != nil {
				*p = Packet{Payload: p.Payload[:0]}
			} else {
				*p = Packet{}
			}
		}
	}
	if p == nil {
		p = &Packet{}
	}
	return
}

// put returns Packet back to pool
// Don't use the Packet and Packet.Payload after a call to put
func (pp *poolPacket) put(p *Packet) {
	if pp.sp != nil {
		pp.sp.Put(p)
	}
}

// putSlice returns every Packet in slice to pool then return the slice itself to poolPacketSlice
// Don't use this objects after a call to putSlice
func (pp *poolPacket) putSlice(ps []*Packet) {
	if pp.sp != nil {
		for i := range ps {
			pp.sp.Put(ps[i])
		}
	}
	poolOfPacketSlice.put(ps)
}

// poolPESData represent fabric class for the PESData objects
// If sync.Pool is used and PESData received via Demuxer.NextData() than you should dispose it yourself
type poolPESData struct {
	sp *sync.Pool
}

// get returns empty PESData
// If sync.Pool is used than PESData may contain Data slice, this slice will be reset to zero length
func (ppd *poolPESData) get() (pd *PESData) {
	if ppd.sp != nil {
		if pd = ppd.sp.Get().(*PESData); pd != nil {
			if pd.Data != nil {
				*pd = PESData{Data: pd.Data[:0]}
			} else {
				*pd = PESData{}
			}
		}
	}
	if pd == nil {
		pd = &PESData{}
	}
	return
}

// put returns PESData back to pool
// Don't use the PESData and PESData.Data after a call to put
func (ppd *poolPESData) put(pd *PESData) {
	if ppd.sp != nil {
		ppd.sp.Put(pd)
	}
}
