package ts

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/asticode/go-astikit"
)

const (
	PTSOrDTSByteLength = 5
	ESCRByteLength     = 6
)

// ClockReference represents a clock reference
// Base is based on a 90 kHz clock and extension is based on a 27 MHz clock
type ClockReference uint64

// NewClockReference builds a new clock reference
func NewClockReference(base, extension uint64) ClockReference {
	return ClockReference((base << 9) | extension&0x1ff)
}

// Duration converts the clock reference into duration
func (cr *ClockReference) Duration() time.Duration {
	return time.Duration(cr.Base()*1e9/90000) + time.Duration(cr.Extension()*1e9/27000000)
}

func (cr *ClockReference) Base() uint64 {
	return uint64(*cr) >> 9
}

func (cr *ClockReference) Extension() uint64 {
	return uint64(*cr) & 0x1ff
}

// Time converts the clock reference into time
func (cr *ClockReference) Time() time.Time {
	return time.Unix(0, cr.Duration().Nanoseconds())
}

// parsePCRBytes parses a Program Clock Reference
// Program clock reference, stored as 33 bits base, 6 bits reserved, 9 bits extension.
func parsePCRBytes(b []byte) ClockReference {
	pcr := uint64(binary.BigEndian.Uint32(b[:4]))<<16 | uint64(binary.BigEndian.Uint32(b[2:6]))
	return NewClockReference(pcr>>15, pcr&0x1ff)
}

func (cr *ClockReference) putPCRBytes(bs []byte) int {
	var bb [8]byte
	binary.BigEndian.PutUint64(bb[:], cr.Extension()|cr.Base()<<15|0x7e<<8)
	copy(bs, bb[2:])
	return pcrBytesSize
}

// parsePTSOrDTS parses a PTS or a DTS
func (cr *ClockReference) parsePTSOrDTS(i *astikit.BytesIterator) (err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(5); err != nil || len(bs) < 5 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	*cr = parsePTSOrDTSValue(bs)
	return
}

func parsePTSOrDTSValue(bs []byte) ClockReference {
	return NewClockReference(uint64(bs[0])>>1&0x7<<30|uint64(bs[1])<<22|uint64(bs[2])>>1&0x7f<<15|uint64(bs[3])<<7|uint64(bs[4])>>1&0x7f, 0)
}

func (cr *ClockReference) ParsePTSOrDTSBytes(bs []byte, o int) (n int, err error) {
	if o+PTSOrDTSByteLength > len(bs) {
		return o, ErrShortPacket
	}
	*cr = parsePTSOrDTSValue(bs[o:])
	return o + PTSOrDTSByteLength, nil
}

func (cr *ClockReference) PutPTSOrDTSBytes(bs []byte, flag uint8) int {
	bs[0] = flag<<4 | uint8(cr.Base()>>29) | 1
	bs[1] = uint8(cr.Base() >> 22)
	bs[2] = uint8(cr.Base()>>14) | 1
	bs[3] = uint8(cr.Base() >> 7)
	bs[4] = uint8(cr.Base()<<1) | 1
	return PTSOrDTSByteLength
}

func (cr *ClockReference) ParseESCRBytes(bs []byte, o int) (n int, err error) {
	if o+ESCRByteLength > len(bs) {
		return o, ErrShortPacket
	}
	b := bs[o:]
	escr := uint64(b[0])>>3&0x7<<39 | uint64(b[0])&0x3<<37 | uint64(b[1])<<29 | uint64(b[2])>>3<<24 | uint64(b[2])&0x3<<22 | uint64(b[3])<<14 | uint64(b[4])>>3<<9 | uint64(b[4])&0x3<<7 | uint64(b[5])>>1
	*cr = NewClockReference(escr>>9, escr&0x1ff)
	return o + ESCRByteLength, nil
}

func (cr *ClockReference) PutESCRBytes(bs []byte) int {
	bs[0] = 0xc0 | uint8((cr.Base()>>27)&0x38) | 0x04 | uint8((cr.Base()>>28)&0x03)
	bs[1] = uint8(cr.Base() >> 20)
	bs[2] = uint8((cr.Base()>>13)&0x3) | 0x4 | uint8((cr.Base()>>12)&0xf8)
	bs[3] = uint8(cr.Base() >> 5)
	bs[4] = uint8(cr.Extension()>>7) | 0x4 | uint8(cr.Base()<<3)
	bs[5] = uint8(cr.Extension()<<1) | 0x1
	return ESCRByteLength
}
