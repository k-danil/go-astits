package demux

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v3/ts"
)

// stuffing goes in the adaptation field: payload past PES_packet_length would be reported (§2.4.3.5)
func stuffedPacket(pid uint16, cc byte, pusi bool, payload []byte) []byte {
	p := bytes.Repeat([]byte{0xff}, ts.PacketSize)
	p[0] = syncByte
	p[1] = byte(pid >> 8)
	if pusi {
		p[1] |= 0x40
	}
	p[2] = byte(pid)
	afLen := ts.PacketSize - ts.HeaderSize - 1 - len(payload)
	p[3] = 0x30 | cc
	p[4] = byte(afLen)
	if afLen > 0 {
		p[5] = 0
	}
	copy(p[ts.HeaderSize+1+afLen:], payload)
	return p
}

func nullPackets(n int) (raw []byte) {
	for cc := range n {
		raw = append(raw, payloadPacket(ts.PIDNull, uint8(cc), false, nil)...)
	}
	return
}

func boundedPES(payload string) []byte {
	n := 3 + len(payload)
	return append([]byte{0, 0, 1, 0xc0, byte(n >> 8), byte(n), 0x80, 0, 0}, payload...)
}

func videoPES(payload string) []byte {
	return append([]byte{0, 0, 1, 0xe0, 0, 0, 0x80, 0, 0}, payload...)
}

func sentinelOf(err error) error {
	for _, s := range []error{ts.ErrSyncLoss, ts.ErrHeadlessUnit, ts.ErrPacketMustStartWithASyncByte} {
		if errors.Is(err, s) {
			return s
		}
	}
	return err
}

func TestSyncLossEndsOpenUnits(t *testing.T) {
	const audio, video, pmt uint16 = 0x100, 0x101, 0x300
	head := videoPES("video-head")
	junk := make([]byte, 300)
	stream := slices.Concat(
		psiPacket(ts.PIDPAT, 0, patSection(0, true, pmt)),
		stuffedPacket(audio, 0, true, boundedPES("unit-A")),
		stuffedPacket(video, 0, true, head),
		nullPackets(5),
		junk,
		psiPacket(ts.PIDPAT, 0, patSection(1, true, pmt)),
		stuffedPacket(video, 1, false, []byte("tail")),
		stuffedPacket(audio, 1, true, boundedPES("unit-B")),
		stuffedPacket(video, 2, true, videoPES("video-2")),
		nullPackets(5),
	)
	lossAt := int64(8 * ts.PacketSize)
	tailAt := lossAt + int64(len(junk)) + int64(ts.PacketSize)
	tests := []struct {
		name     string
		opts     []func(*Demuxer)
		wantErrs []ts.RecoverableError
	}{
		{
			name: "reported",
			opts: []func(*Demuxer){WithRecoverableErrors()},
			wantErrs: []ts.RecoverableError{
				{Kind: ts.ErrorKindSyncLoss, PID: ts.PIDUnset, Offset: lossAt, Dropped: int64(len(junk)), Err: ts.ErrPacketMustStartWithASyncByte},
				{Kind: ts.ErrorKindContinuity, PID: audio, Offset: lossAt, Err: ts.ErrSyncLoss},
				{Kind: ts.ErrorKindTornUnit, PID: video, Offset: lossAt, Dropped: int64(len(head)), Err: ts.ErrSyncLoss},
				{Kind: ts.ErrorKindTornUnit, PID: video, Offset: tailAt, Dropped: 4, Err: ts.ErrHeadlessUnit},
			},
		},
		{name: "unreported"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := append([]func(*Demuxer){WithPacketSize(ts.PacketSize), WithSyncLock(), WithSkipErrLimit(16), WithResyncLimit(-1)}, tt.opts...)
			dmx := New(context.Background(), bytes.NewReader(stream), opts...)
			defer dmx.Close()
			var events []string
			var errs []ts.RecoverableError
			for ev, err := range dmx.Events() {
				if re, ok := errors.AsType[*ts.RecoverableError](err); ok {
					e := *re
					e.Err = sentinelOf(e.Err)
					errs = append(errs, e)
					continue
				}
				require.NoError(t, err)
				switch ev {
				case EventPES:
					d := dmx.PES()
					events = append(events, string(d.Data.Data))
					d.Close()
				case EventPAT:
					events = append(events, "PAT")
				}
			}
			assert.Equal(t, []string{"PAT", "unit-A", "PAT", "unit-B", "video-2"}, events)
			assert.Equal(t, tt.wantErrs, errs)
		})
	}
}
