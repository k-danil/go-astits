package demux

import (
	"math/bits"
	"sync"
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
			New: func() any {
				return &dataPayload{
					bs: make([]byte, 0, s),
				}
			},
		}
	}
	return p
}

// getClass returns a payload object with an empty slice backed by at least
// the class capacity.
func (ptp *poolPayload) getClass(class uint8) (dp *dataPayload) {
	if int(class) < len(ptp.sp) {
		dp, _ = ptp.sp[class].Get().(*dataPayload)
		dp.bs = dp.bs[:0]
		return
	}
	return &dataPayload{
		bs: make([]byte, 0, 1024<<class),
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
