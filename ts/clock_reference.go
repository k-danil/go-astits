package ts

import (
	"encoding/binary"
	"time"
)

const (
	PTSDTSSize = 5
	ESCRSize   = 6
	PCRSize    = 6
)

const (
	// PTS and DTS tick at ClockHz/PTSTicks (90 kHz).
	ClockHz   = 27_000_000
	PTSTicks  = 300
	ClockWrap = (1 << 33) * PTSTicks
)

// A count of ClockHz ticks; PTS and DTS are multiples of PTSTicks.
type ClockReference int64

// base is the 33-bit 90 kHz field, extension the 27 MHz remainder (0..299); a
// PTS or DTS has none.
func NewClockReference(base, extension uint64) ClockReference {
	return ClockReference(base*PTSTicks + extension)
}

func (cr ClockReference) Base() uint64 {
	return uint64(cr) / PTSTicks
}

func (cr ClockReference) Extension() uint64 {
	return uint64(cr) % PTSTicks
}

func (cr ClockReference) Duration() time.Duration {
	// cr*1e9 overflows int64 near the wrap: split at the microsecond (27 ticks).
	const ticksPerMicro = ClockHz / 1_000_000
	return time.Duration(cr/ticksPerMicro)*time.Microsecond + time.Duration(cr%ticksPerMicro)*time.Microsecond/ticksPerMicro
}

// cr - o as the shortest signed distance across ClockWrap.
func (cr ClockReference) Diff(o ClockReference) ClockReference {
	d := cr - o
	switch {
	case d >= ClockWrap/2:
		d -= ClockWrap
	case d < -ClockWrap/2:
		d += ClockWrap
	}
	return d
}

// PCR is 33 bits base, 6 reserved, 9 extension.
func (cr *ClockReference) ParsePCR(bs []byte) (n int, err error) {
	if len(bs) < PCRSize {
		return 0, ErrShortPacket
	}
	pcr := uint64(binary.BigEndian.Uint32(bs[:4]))<<16 | uint64(binary.BigEndian.Uint32(bs[2:6]))
	*cr = NewClockReference(pcr>>15, pcr&0x1ff)
	return PCRSize, nil
}

func (cr ClockReference) PutPCR(bs []byte) (n int) {
	var bb [8]byte
	binary.BigEndian.PutUint64(bb[:], cr.Extension()|cr.Base()<<15|0x7e<<8)
	copy(bs, bb[2:])
	return PCRSize
}

func (cr *ClockReference) ParsePTSDTS(bs []byte) (n int, err error) {
	if len(bs) < PTSDTSSize {
		return 0, ErrShortPacket
	}
	// PTS: 3 bits [32:30], 15 [29:15], 15 [14:0], each after a marker bit.
	v := uint64(binary.BigEndian.Uint32(bs[:4]))<<8 | uint64(bs[4])
	*cr = NewClockReference(v>>33&0x7<<30|v>>17&0x7fff<<15|v>>1&0x7fff, 0)
	return PTSDTSSize, nil
}

func (cr ClockReference) PutPTSDTS(bs []byte, flag uint8) (n int) {
	bs[0] = flag<<4 | uint8(cr.Base()>>29) | 1
	bs[1] = uint8(cr.Base() >> 22)
	bs[2] = uint8(cr.Base()>>14) | 1
	bs[3] = uint8(cr.Base() >> 7)
	bs[4] = uint8(cr.Base()<<1) | 1
	return PTSDTSSize
}

func (cr *ClockReference) ParseESCR(bs []byte) (n int, err error) {
	if len(bs) < ESCRSize {
		return 0, ErrShortPacket
	}
	escr := uint64(bs[0])>>3&0x7<<39 | uint64(bs[0])&0x3<<37 | uint64(bs[1])<<29 | uint64(bs[2])>>3<<24 | uint64(bs[2])&0x3<<22 | uint64(bs[3])<<14 | uint64(bs[4])>>3<<9 | uint64(bs[4])&0x3<<7 | uint64(bs[5])>>1
	*cr = NewClockReference(escr>>9, escr&0x1ff)
	return ESCRSize, nil
}

func (cr ClockReference) PutESCR(bs []byte) (n int) {
	bs[0] = 0xc0 | uint8((cr.Base()>>27)&0x38) | 0x04 | uint8((cr.Base()>>28)&0x03)
	bs[1] = uint8(cr.Base() >> 20)
	bs[2] = uint8((cr.Base()>>13)&0x3) | 0x4 | uint8((cr.Base()>>12)&0xf8)
	bs[3] = uint8(cr.Base() >> 5)
	bs[4] = uint8(cr.Extension()>>7) | 0x4 | uint8(cr.Base()<<3)
	bs[5] = uint8(cr.Extension()<<1) | 0x1
	return ESCRSize
}
