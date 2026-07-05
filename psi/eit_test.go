package psi

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/k-danil/go-astits/v2/internal/bitstest"
	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

var eit = &EIT{
	Events: []EITEvent{{
		Descriptors:    descriptors,
		Duration:       dvbSecondsDuration,
		EventID:        6,
		HasFreeCSAMode: true,
		RunningStatus:  7,
		StartTime:      dvbTime,
	}},
	LastTableID:              5,
	OriginalNetworkID:        3,
	SegmentLastSectionNumber: 4,
	ServiceID:                1,
	TransportStreamID:        2,
}

func eitBytes() []byte {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write(uint16(2))               // Transport stream ID
	_ = w.Write(uint16(3))               // Original network ID
	_ = w.Write(uint8(4))                // Segment last section number
	_ = w.Write(uint8(5))                // Last table id
	_ = w.Write(uint16(6))               // Event #1 id
	_ = w.Write(dvbTimeBytes)            // Event #1 start time
	_ = w.Write(dvbSecondsDurationBytes) // Event #1 duration
	_ = w.Write("111")                   // Event #1 running status
	_ = w.Write("1")                     // Event #1 free CA mode
	descriptorsBytes(w)                  // Event #1 descriptors
	return buf.Bytes()
}

func TestParseEITSection(t *testing.T) {
	var b = eitBytes()
	d, err := parseEITSection(bytesiter.New(b), len(b), uint16(1))
	assert.Equal(t, d, eit)
	assert.NoError(t, err)
}
