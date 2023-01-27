package astits

import "sync"

var poolOfPacketSlices = &packetSlicesPool{
	sp: sync.Pool{
		New: func() interface{} {
			// Prepare slice of somewhat sensible initial size to minimise calls to runtime.growslice
			ps := make([]*Packet, 0, 64)
			return &ps
		},
	},
}

type packetSlicesPool struct {
	sp sync.Pool
}

func (psp *packetSlicesPool) get() []*Packet {
	// Reset slice length to use with append
	return (*(psp.sp.Get().(*[]*Packet)))[:0]
}

func (psp *packetSlicesPool) put(ps []*Packet) {
	psp.sp.Put(&ps)
}

var poolOfData = &tempDataPool{
	sp: sync.Pool{
		New: func() interface{} {
			// Prepare slice of somewhat sensible initial size to minimise calls to runtime.growslice
			d := make([]byte, 0, 1024)
			return &d
		},
	},
}

type tempDataPool struct {
	sp sync.Pool
}

func (tdp *tempDataPool) get(size int) (payload []byte) {
	payload = *(tdp.sp.Get().(*[]byte))
	// Reset slice length or grow it to requested size to use with copy
	if cap(payload) >= size {
		payload = payload[:size]
	} else {
		n := size - cap(payload)
		payload = append(payload[:cap(payload)], make([]byte, n)...)[:size]
	}
	return
}

func (tdp *tempDataPool) put(payload []byte) {
	tdp.sp.Put(&payload)
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
