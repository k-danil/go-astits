package astits

import "sync"

// poolOfTempPayload global variable is used to ease access to pool from any place of the code
var poolOfTempPayload = &poolTempPayload{
	sp: sync.Pool{
		New: func() interface{} {
			// Prepare the slice of somewhat sensible initial size to minimize calls to runtime.growslice
			return &tempPayload{
				s: make([]byte, 0, 1<<13),
			}
		},
	},
}

// PoolOfPESData global variable is used to ease access to pool from any place of the code
var PoolOfPESData = &poolPESData{
	sp: sync.Pool{
		New: func() interface{} {
			return &PESData{Data: make([]byte, 0, 1<<13)}
		},
	},
}

// tempPayload is an object containing payload slice
type tempPayload struct {
	s []byte
}

// poolTempPayload is a pool for temporary payload in parseData()
// Don't use it anywhere else to avoid pool pollution
type poolTempPayload struct {
	sp sync.Pool
}

// get returns the tempPayload object with byte slice of a 'size' length
func (ptp *poolTempPayload) get(size int) (payload *tempPayload) {
	payload = ptp.sp.Get().(*tempPayload)
	// Reset slice length or grow it to requested size for use with copy
	if cap(payload.s) >= size {
		payload.s = payload.s[:size]
	} else {
		n := size - cap(payload.s)
		payload.s = append(payload.s[:cap(payload.s)], make([]byte, n)...)[:size]
	}
	return
}

// put returns reference to the payload slice back to pool
// Don't use the payload after a call to put
func (ptp *poolTempPayload) put(payload *tempPayload) {
	ptp.sp.Put(payload)
}

// poolPESData represent fabric class for the PESData objects
// If sync.Pool is used and PESData received via Demuxer.NextData() than you should dispose it yourself
type poolPESData struct {
	sp sync.Pool
}

// get returns empty PESData
// If sync.Pool is used than PESData may contain Data slice, this slice will be reset to zero length
func (ppd *poolPESData) get() (pd *PESData) {
	if pd = ppd.sp.Get().(*PESData); pd != nil {
		if pd.Data != nil {
			*pd = PESData{Data: pd.Data[:0]}
		} else {
			*pd = PESData{}
		}
	}
	if pd == nil {
		pd = &PESData{}
	}
	return
}

// Put returns PESData back to pool
// Don't use the PESData and PESData.Data after a call to Put
func (ppd *poolPESData) Put(pd *PESData) {
	ppd.sp.Put(pd)
}

// PutSlice returns every Packet in the slice to pool then return the slice itself to poolPacketSlice
// Don't use this objects after a call to PutSlice
func (ppd *poolPESData) PutSlice(pd []*PESData) {
	for i := range pd {
		ppd.sp.Put(pd[i])
	}
}
