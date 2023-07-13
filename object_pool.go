package astits

import (
	"math/bits"
	"sync"
	"unsafe"
)

// poolOfTempPayload global variable is used to ease access to pool from any place of the code
var poolOfTempPayload = initPool()

// tempPayload is an object containing payload slice
type tempPayload struct {
	s []byte
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
				return &tempPayload{
					s: make([]byte, 0, s),
				}
			},
		}
	}
	return p
}

// get returns the tempPayload object with byte slice of a 'size' length
func (ptp *poolTempPayload) get(size int) (payload *tempPayload) {
	s := uint(size)
	idx := bits.Len(s) - 10
	neg := idx >= 0
	idx *= int(*(*uint8)(unsafe.Pointer(&neg)))
	if idx < len(ptp.sp) && idx >= 0 {
		payload, _ = ptp.sp[idx].Get().(*tempPayload)
		if uint(cap(payload.s)) >= s {
			payload.s = payload.s[:s]
		}
		return
	}

	return &tempPayload{
		s: make([]byte, s),
	}
}

// put returns reference to the payload slice back to pool
// Don't use the payload after a call to put
func (ptp *poolTempPayload) put(payload *tempPayload) {
	c := uint(cap(payload.s))
	idx := bits.Len(c) - 11
	if idx < len(ptp.sp) && idx >= 0 {
		ptp.sp[idx].Put(payload)
	}
}
