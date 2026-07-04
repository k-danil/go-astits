package psi

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/k-danil/go-astits/v2/internal/bitstest"
	"github.com/k-danil/go-astits/v2/internal/bytesiter"
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
	w.Write(uint16(2))  // Original network ID
	w.Write(uint8(0))   // Reserved for future use
	w.Write(uint16(3))  // Service #1 id
	w.Write("000000")   // Service #1 reserved for future use
	w.Write("1")        // Service #1 EIT schedule flag
	w.Write("1")        // Service #1 EIT present/following flag
	w.Write("101")      // Service #1 running status
	w.Write("1")        // Service #1 free CA mode
	descriptorsBytes(w) // Service #1 descriptors
	return buf.Bytes()
}

func TestParseSDTSection(t *testing.T) {
	var b = sdtBytes()
	d, err := parseSDTSection(bytesiter.New(b), len(b), uint16(1))
	assert.Equal(t, d, sdt)
	assert.NoError(t, err)
}
