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

var poolOfPackets = &packetPoolS{}

type packetPoolS struct {
	sp *sync.Pool
}

func (pps *packetPoolS) get() (p *Packet) {
	if pps.sp != nil {
		if p = pps.sp.Get().(*Packet); p != nil {
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

func (pps *packetPoolS) put(p *Packet) {
	if pps.sp != nil {
		pps.sp.Put(p)
	}
}

func (pps *packetPoolS) putSlice(ps []*Packet) {
	if pps.sp != nil {
		for i := range ps {
			pps.sp.Put(ps[i])
		}
	}
	poolOfPacketSlices.put(ps)
}

var poolOfPESData = pesPool{}

type pesPool struct {
	sp *sync.Pool
}

func (pp *pesPool) get() (pd *PESData) {
	if pp.sp != nil {
		if pd = pp.sp.Get().(*PESData); pd != nil {
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
