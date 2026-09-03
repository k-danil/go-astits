package ts_test

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/ts"
)

// Build a packet by hand and serialize it: Put stuffs the tail to the packet
// size.
func ExamplePacket_Put() {
	p := ts.NewPacket()
	defer p.Close()
	p.Header = ts.PacketHeader{PID: 0x100, PayloadUnitStartIndicator: true, HasPayload: true, HasAdaptationField: true, ContinuityCounter: 3}
	p.SetAdaptationField(&ts.PacketAdaptationField{HasPCR: true, PCR: ts.NewClockReference(90000, 0), RandomAccessIndicator: true})
	p.Payload = []byte("payload")

	bs := make([]byte, ts.PacketSize)
	n, err := p.Put(bs)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%d bytes, sync 0x%02x, pid 0x%x\n", n, bs[0], uint16(bs[1]&0x1f)<<8|uint16(bs[2]))
	// Output: 188 bytes, sync 0x47, pid 0x100
}

// A ClockReference counts 27 MHz ticks: PCR carries a 90 kHz base plus a
// 27 MHz extension, PTS and DTS only the base. Diff survives the 33-bit wrap.
func ExampleNewClockReference() {
	pcr := ts.NewClockReference(90000, 150)
	pts := ts.NewClockReference(90001, 0)
	fmt.Println(pcr.Base(), pcr.Extension(), pcr.Duration())
	fmt.Println(pts.Diff(pcr), "ticks =", pts.Diff(pcr).Duration())

	beforeWrap := ts.ClockReference(ts.ClockWrap - 300)
	afterWrap := ts.ClockReference(300)
	fmt.Println(afterWrap.Diff(beforeWrap), beforeWrap.Diff(afterWrap))
	// Output:
	// 90000 150 1.000005555s
	// 150 ticks = 5.555µs
	// 600 -600
}

// A PIDSet is a 1 KiB bit set; pass it by pointer.
func ExampleNewPIDSet() {
	keep := ts.NewPIDSet(ts.PIDPAT, 0x100)
	keep.Add(0x101)
	fmt.Println(keep.Has(0x100), keep.Has(0x101), keep.Has(0x200))
	// Output: true true false
}
