package demux

import (
	"math/bits"
	"sync"
)

var poolOfPayload = initPool()

type dataPayload struct {
	bs []byte
}

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

func (ptp *poolPayload) put(dp *dataPayload) {
	c := uint(cap(dp.bs))
	idx := bits.Len(c) - 11
	if idx < len(ptp.sp) && idx >= 0 {
		ptp.sp[idx].Put(dp)
	}
}
