package demux

import (
	"bytes"

	"github.com/k-danil/go-astits/v3/ts"
)

func offsetTestStream(pids []uint16) []byte {
	buf := &bytes.Buffer{}
	cc := make(map[uint16]uint8)
	for _, pid := range pids {
		b, _ := packetShort(ts.PacketHeader{
			ContinuityCounter: cc[pid],
			HasPayload:        true,
			PID:               pid,
		}, []byte{0xde, 0xad, 0xbe, 0xef})
		cc[pid] = (cc[pid] + 1) & 0xf
		// packetShort pads without accounting for the header and returns 192 bytes
		buf.Write(b[:ts.PacketSize])
	}
	return buf.Bytes()
}
