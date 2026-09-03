package psi

import (
	"bytes"

	"github.com/k-danil/go-astits/v3/internal/bitstest"
)

var sdt = &SDT{
	OriginalNetworkID: 2,
	Services: []SDTService{{
		Descriptors:            descriptors,
		HasEITPresentFollowing: true,
		HasEITSchedule:         true,
		HasFreeCSAMode:         true,
		RunningStatus:          5,
		ServiceID:              3,
	}},
	TransportStreamID: 1,
}

func sdtBytes() []byte {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write(uint16(2)) // Original network ID
	_ = w.Write(uint8(0))  // Reserved for future use
	_ = w.Write(uint16(3)) // Service #1 id
	_ = w.Write("000000")  // Service #1 reserved for future use
	_ = w.Write("1")       // Service #1 EIT schedule flag
	_ = w.Write("1")       // Service #1 EIT present/following flag
	_ = w.Write("101")     // Service #1 running status
	_ = w.Write("1")       // Service #1 free CA mode
	descriptorsBytes(w)    // Service #1 descriptors
	return buf.Bytes()
}
