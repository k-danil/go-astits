package astits

import (
	"math/bits"
	"sync"
	"unsafe"
)

// poolOfPayload global variable is used to ease access to pool from any place of the code
var poolOfPayload = initPool()

// dataPayload is an object containing payload slice
type dataPayload struct {
	bs []byte
}

// poolPayload is a pool for temporary payload in parseData()
// Don't use it anywhere else to avoid pool pollution
type poolPayload struct {
	sp [16]sync.Pool
}

func initPool() *poolPayload {
	p := &poolPayload{}
	for i := range p.sp {
		s := (1 << i) * 1024
		p.sp[i] = sync.Pool{
			New: func() interface{} {
				return &dataPayload{
					bs: make([]byte, 0, s),
				}
			},
		}
	}
	return p
}

// get returns the payload object with byte slice of a 'size' length
func (ptp *poolPayload) get(size int) (dp *dataPayload) {
	s := uint(size)
	idx := bits.Len(s) - 10
	neg := idx >= 0
	idx *= int(*(*uint8)(unsafe.Pointer(&neg)))
	if idx < len(ptp.sp) && idx >= 0 {
		dp, _ = ptp.sp[idx].Get().(*dataPayload)
		if uint(cap(dp.bs)) >= s {
			dp.bs = dp.bs[:s]
		}
		return
	}

	return &dataPayload{
		bs: make([]byte, s),
	}
}

// put returns reference to the payload slice back to pool
// Don't use the payload after a call to put
func (ptp *poolPayload) put(dp *dataPayload) {
	c := uint(cap(dp.bs))
	idx := bits.Len(c) - 11
	if idx < len(ptp.sp) && idx >= 0 {
		ptp.sp[idx].Put(dp)
	}
}
