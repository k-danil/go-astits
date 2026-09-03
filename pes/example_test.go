package pes_test

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/pes"
	"github.com/k-danil/go-astits/v3/ts"
)

// Serialize a PES header for a payload, then parse the packet back.
func ExampleData_Parse() {
	payload := []byte("access unit")
	h := pes.Header{
		StreamID:       0xe0, // a video stream id
		OptionalHeader: &pes.OptionalHeader{PTSDTSIndicator: pes.PTSDTSIndicatorOnlyPTS, PTS: ts.NewClockReference(90000, 0)},
	}
	buf := make([]byte, 64)
	n, err := h.PutHeader(buf, len(payload))
	if err != nil {
		panic(err)
	}

	var d pes.Data
	if err = d.Parse(append(buf[:n], payload...)); err != nil {
		panic(err)
	}
	fmt.Println(d.Header.OptionalHeader.PTS.Duration(), string(d.Data))
	// Output: 1s access unit
}
