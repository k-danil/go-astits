package ts

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzPacketParse(f *testing.F) {
	b, _ := packet([]byte("payload"), false)
	f.Add(b)
	b, _ = packet([]byte("payload"), true)
	f.Add(b)
	b, _ = packetShort(PacketHeader{HasPayload: true, PID: 0x100}, []byte{0xde})
	f.Add(b[:PacketSize])
	f.Add(make([]byte, PacketSize))
	f.Fuzz(func(t *testing.T, bs []byte) {
		p := NewPacket()
		defer p.Close()
		_, _ = p.parse(bs, nil, nil)
	})
}

func FuzzAdaptationFieldParse(f *testing.F) {
	f.Add(packetAdaptationFieldBytes())
	f.Add([]byte{0x00})
	f.Add([]byte{0x01, 0x40})
	f.Fuzz(func(t *testing.T, bs []byte) {
		var af PacketAdaptationField
		_, _ = af.Parse(bs)
	})
}

func fuzzStream(packets, size, prefixLen int) []byte {
	bs := make([]byte, packets*size)
	for i := range packets {
		off := i*size + prefixLen
		bs[off] = syncByte
		bs[off+1] = 0x01
		bs[off+2] = 0x00 // PID 0x100
		bs[off+3] = 0x10 | byte(i&0xf)
	}
	return bs
}

// Both budgets open, so any damage is recovered from and the run can only end at the input's end.
func FuzzPacketBufferSyncLock(f *testing.F) {
	brokenSync := fuzzStream(8, PacketSize, 0)
	brokenSync[3*PacketSize] = 0x00

	f.Add(fuzzStream(8, PacketSize, 0), uint8(0), uint8(0))
	f.Add(fuzzStream(8, M2TSPacketSize, m2tsPrefixSize), uint8(1), uint8(2))
	f.Add(fuzzStream(8, RSPacketSize, 0), uint8(2), uint8(3))
	f.Add(brokenSync, uint8(3), uint8(1))
	f.Add(bytes.Repeat([]byte{0xa5}, 1000), uint8(0), uint8(0))

	const maxPackets = 4096
	f.Fuzz(func(t *testing.T, stream []byte, flags, sizeSel uint8) {
		cfg := PacketBufferConfig{SyncLock: true, SkipErrLimit: -1, ResyncLimit: -1}
		switch sizeSel % 4 {
		case 1:
			cfg.PacketSize = PacketSize
		case 2:
			cfg.PacketSize = M2TSPacketSize
		case 3:
			cfg.PacketSize = RSPacketSize
		}
		if flags&1 != 0 {
			cfg.ZeroCopyBatch = 8
		}
		if flags&2 != 0 {
			keep := NewPIDSet(0x100)
			cfg.KeepPIDs = &keep
		}

		pb, err := NewPacketBuffer(context.Background(), bytes.NewReader(stream), cfg)
		if err != nil {
			return
		}
		p := NewPacket()
		defer p.Close()
		pos := pb.Pos()
		for range maxPackets {
			if err = pb.Next(p); err != nil {
				require.ErrorIs(t, err, ErrNoMorePackets)
				break
			}
			require.Greater(t, pb.Pos(), pos)
			require.LessOrEqual(t, pb.Pos(), int64(len(stream)))
			require.Len(t, p.Raw(), int(pb.PacketSize()))
			pos = pb.Pos()
		}
	})
}

func FuzzClockParse(f *testing.F) {
	f.Add(pcrBytes())
	f.Fuzz(func(t *testing.T, bs []byte) {
		var cr ClockReference
		_, _ = cr.ParsePCR(bs)
		_, _ = cr.ParsePTSDTS(bs)
		_, _ = cr.ParseESCR(bs)
	})
}
