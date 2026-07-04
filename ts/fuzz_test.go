package ts

import (
	"testing"
)

func FuzzPacketParse(f *testing.F) {
	b, _ := packet(packetHeader, packetAdaptationField, []byte("payload"), false)
	f.Add(b)
	b, _ = packet(packetHeader, packetAdaptationField, []byte("payload"), true)
	f.Add(b)
	b, _ = packetShort(PacketHeader{HasPayload: true, PID: 0x100}, []byte{0xde})
	f.Add(b[:PacketSize])
	f.Add(make([]byte, PacketSize))
	f.Fuzz(func(t *testing.T, bs []byte) {
		p := NewPacket()
		defer p.Close()
		p.parse(bs, EmptySkipper)
	})
}

func FuzzAdaptationFieldParse(f *testing.F) {
	f.Add(packetAdaptationFieldBytes(packetAdaptationField))
	f.Add([]byte{0x00})
	f.Add([]byte{0x01, 0x40})
	f.Fuzz(func(t *testing.T, bs []byte) {
		var af PacketAdaptationField
		af.Parse(bs)
	})
}

func FuzzClockParse(f *testing.F) {
	f.Add(pcrBytes())
	f.Fuzz(func(t *testing.T, bs []byte) {
		var cr ClockReference
		cr.ParsePCR(bs)
		cr.ParsePTSDTS(bs)
		cr.ParseESCR(bs)
	})
}
