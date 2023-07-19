package astits

import (
	"math/bits"
	"sync"
	"unsafe"
)

// poolOfPayload global variable is used to ease access to pool from any place of the code
var poolOfPayload = initPool()

// payload is an object containing payload slice
type payload struct {
	bs []byte
}

// poolTempPayload is a pool for temporary payload in parseData()
// Don't use it anywhere else to avoid pool pollution
type poolTempPayload struct {
	sp [16]sync.Pool
}

func initPool() *poolTempPayload {
	p := &poolTempPayload{}
	for i := range p.sp {
		s := (1 << i) * 1024
		p.sp[i] = sync.Pool{
			New: func() interface{} {
				return &payload{
					bs: make([]byte, 0, s),
				}
			},
		}
	}
	return p
}

// get returns the payload object with byte slice of a 'size' length
func (ptp *poolTempPayload) get(size int) (p *payload) {
	s := uint(size)
	idx := bits.Len(s) - 10
	neg := idx >= 0
	idx *= int(*(*uint8)(unsafe.Pointer(&neg)))
	if idx < len(ptp.sp) && idx >= 0 {
		p, _ = ptp.sp[idx].Get().(*payload)
		if uint(cap(p.bs)) >= s {
			p.bs = p.bs[:s]
		}
		return
	}

	return &payload{
		bs: make([]byte, s),
	}
}

// put returns reference to the payload slice back to pool
// Don't use the payload after a call to put
func (ptp *poolTempPayload) put(payload *payload) {
	c := uint(cap(payload.bs))
	idx := bits.Len(c) - 11
	if idx < len(ptp.sp) && idx >= 0 {
		ptp.sp[idx].Put(payload)
	}
}
