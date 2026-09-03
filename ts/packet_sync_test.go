package ts

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// syncPacket builds one minimal valid 188-byte TS packet (sync, has-payload,
// zero payload) — enough for p.parse to accept it.
func syncPacket() []byte {
	p := make([]byte, PacketSize)
	p[0] = syncByte
	p[3] = 0x10 // payload present, no adaptation field
	return p
}

func syncPackets(n int) []byte {
	var b []byte
	for range n {
		b = append(b, syncPacket()...)
	}
	return b
}

// drainSync reads every packet a sync-locked buffer yields, returning their
// byte offsets and the terminating error.
func drainSync(t *testing.T, r *bytes.Reader, cfg PacketBufferConfig) (offsets []int64, err error) {
	t.Helper()
	pb, nerr := NewPacketBuffer(context.Background(), r, cfg)
	require.NoError(t, nerr)
	p := NewPacket()
	for {
		if err = pb.Next(p); err != nil {
			return
		}
		offsets = append(offsets, p.Offset)
	}
}

// lossy is the sync-lock configuration a torn feed needs: both budgets open.
var lossy = PacketBufferConfig{SyncLock: true, SkipErrLimit: -1, ResyncLimit: -1}

// corruptPacket has a valid sync byte but an adaptation-field length that
// overruns the packet, so it aligns yet fails to parse.
func corruptPacket() []byte {
	p := make([]byte, PacketSize)
	p[0] = syncByte
	p[3] = 0x20 // adaptation field present, no payload
	p[4] = 200  // AF length > packet body → ErrShortPacket
	return p
}

// Re-locking needs exactly five consecutive periods (TR 101 290 hysteresis):
// a shorter island of packets inside damage stays part of the loss instead of
// resurfacing as a sync and tearing the stream a second time; five lock.
func TestResyncNeedsFivePeriods(t *testing.T) {
	const junk = 100
	tests := []struct {
		name    string
		island  int
		packets int
		dropped []int64
	}{
		{"island of four is part of the loss", 4, 11, []int64{junk + 4*PacketSize + junk}},
		{"island of five locks", 5, 16, []int64{junk, junk}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stream []byte
			stream = append(stream, syncPackets(5)...)
			stream = append(stream, make([]byte, junk)...)
			stream = append(stream, syncPackets(tt.island)...)
			stream = append(stream, make([]byte, junk)...)
			stream = append(stream, syncPackets(6)...)

			var got []RecoverableError
			cfg := lossy
			cfg.OnRecover = func(e RecoverableError) { got = append(got, e) }
			offsets, err := drainSync(t, bytes.NewReader(stream), cfg)
			require.ErrorIs(t, err, ErrNoMorePackets)
			require.Len(t, offsets, tt.packets)
			assert.Equal(t, int64(5*PacketSize+junk+tt.island*PacketSize+junk), offsets[len(offsets)-6], "the six trailing packets sit past both junk runs")

			var dropped []int64
			for _, e := range got {
				require.Equal(t, ErrorKindSyncLoss, e.Kind)
				dropped = append(dropped, e.Dropped)
			}
			assert.Equal(t, tt.dropped, dropped)
		})
	}
}

// Bytes before the first lock are a loss like any other: reported with their
// count, not silently discarded.
func TestSyncLockLeadingLossReported(t *testing.T) {
	var stream []byte
	stream = append(stream, syncPacket()...)
	hit := syncPacket()
	hit[0] = 0x00
	stream = append(stream, hit...)
	stream = append(stream, syncPackets(4)...)

	var got []RecoverableError
	cfg := PacketBufferConfig{SyncLock: true, OnRecover: func(e RecoverableError) { got = append(got, e) }}
	offsets, err := drainSync(t, bytes.NewReader(stream), cfg)
	require.ErrorIs(t, err, ErrNoMorePackets)
	assert.Equal(t, []int64{376, 564, 752, 940}, offsets, "the lock lands on the first three clean periods")
	require.Len(t, got, 1)
	assert.Equal(t, ErrorKindSyncLoss, got[0].Kind)
	assert.Equal(t, int64(0), got[0].Offset)
	assert.Equal(t, int64(376), got[0].Dropped)
}

// An exhausted streak still reports the packet that broke it before the fatal,
// in both modes, so the loss is accounted for like every other.
func TestStreakFatalReportsTheDrop(t *testing.T) {
	badSync := syncPacket()
	badSync[0] = 0x00
	tests := []struct {
		name   string
		cfg    PacketBufferConfig
		stream []byte
	}{
		{"plain", PacketBufferConfig{PacketSize: PacketSize}, append(syncPackets(3), badSync...)},
		{"sync lock", PacketBufferConfig{SyncLock: true}, append(syncPackets(3), corruptPacket()...)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []RecoverableError
			cfg := tt.cfg
			cfg.OnRecover = func(e RecoverableError) { got = append(got, e) }
			offsets, err := drainSync(t, bytes.NewReader(tt.stream), cfg)
			require.ErrorIs(t, err, ErrInvalidData)
			assert.Len(t, offsets, 3)
			require.Len(t, got, 1)
			assert.Equal(t, ErrorKindPacketDrop, got[0].Kind)
			assert.Equal(t, int64(3*PacketSize), got[0].Offset)
			assert.Equal(t, int64(PacketSize), got[0].Dropped)
		})
	}
}

// Both budgets share one scale: 0 tolerates nothing, -1 never gives up, N
// allows N in a row. The streak counts drops in either mode and a completed
// resync, and a clean packet resets it; the resync limit counts scan windows.
func TestDamageLimits(t *testing.T) {
	badSync := func() []byte {
		p := syncPacket()
		p[0] = 0x00
		return p
	}
	repeat := func(n int, pkt func() []byte) (b []byte) {
		for range n {
			b = append(b, pkt()...)
		}
		return
	}
	junk := func(n int) []byte { return make([]byte, n) }
	cat := func(parts ...[]byte) (b []byte) {
		for _, p := range parts {
			b = append(b, p...)
		}
		return
	}

	streakPlain := cat(syncPackets(3), repeat(3, badSync), syncPackets(3))
	streakLocked := cat(syncPackets(3), repeat(3, corruptPacket), syncPackets(3))
	resetLocked := cat(syncPackets(1), corruptPacket(), syncPackets(1), corruptPacket(), syncPackets(1))
	lossThenDrop := cat(syncPackets(5), junk(100), corruptPacket(), syncPackets(5))
	lossCleanDrop := cat(syncPackets(5), junk(100), syncPackets(1), corruptPacket(), syncPackets(5))
	longLoss := cat(syncPackets(3), junk(3*syncScanWindow), syncPackets(6))
	longLossThenDrop := cat(syncPackets(3), junk(3*syncScanWindow), corruptPacket(), syncPackets(5))

	tests := []struct {
		name    string
		cfg     PacketBufferConfig
		stream  []byte
		packets int
		fatal   bool
	}{
		{"plain skip 0", PacketBufferConfig{PacketSize: PacketSize, SkipErrLimit: 0}, streakPlain, 3, true},
		{"plain skip -1", PacketBufferConfig{PacketSize: PacketSize, SkipErrLimit: -1}, streakPlain, 6, false},
		{"plain skip 2", PacketBufferConfig{PacketSize: PacketSize, SkipErrLimit: 2}, streakPlain, 3, true},
		{"locked skip 0", PacketBufferConfig{SyncLock: true, SkipErrLimit: 0, ResyncLimit: -1}, streakLocked, 3, true},
		{"locked skip -1", PacketBufferConfig{SyncLock: true, SkipErrLimit: -1, ResyncLimit: -1}, streakLocked, 6, false},
		{"locked skip 2", PacketBufferConfig{SyncLock: true, SkipErrLimit: 2, ResyncLimit: -1}, streakLocked, 3, true},
		{"clean packet resets the streak", PacketBufferConfig{SyncLock: true, SkipErrLimit: 1, ResyncLimit: -1}, resetLocked, 3, false},
		{"resync counts once, drop right after is fatal", PacketBufferConfig{SyncLock: true, SkipErrLimit: 1, ResyncLimit: -1}, lossThenDrop, 5, true},
		{"resync counts once, clean packet between", PacketBufferConfig{SyncLock: true, SkipErrLimit: 1, ResyncLimit: -1}, lossCleanDrop, 11, false},
		{"resync over many windows still counts once", PacketBufferConfig{SyncLock: true, SkipErrLimit: 2, ResyncLimit: -1}, longLossThenDrop, 8, false},
		{"resync 0: loss is fatal at once", PacketBufferConfig{SyncLock: true, SkipErrLimit: -1, ResyncLimit: 0}, longLoss, 3, true},
		{"resync -1: scans to the lock", PacketBufferConfig{SyncLock: true, SkipErrLimit: -1, ResyncLimit: -1}, longLoss, 9, false},
		{"resync 1: one fruitless window", PacketBufferConfig{SyncLock: true, SkipErrLimit: -1, ResyncLimit: 1}, longLoss, 3, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offsets, err := drainSync(t, bytes.NewReader(tt.stream), tt.cfg)
			if tt.fatal {
				require.ErrorIs(t, err, ErrInvalidData)
				require.NotErrorIs(t, err, ErrNoMorePackets)
			} else {
				require.ErrorIs(t, err, ErrNoMorePackets)
			}
			assert.Len(t, offsets, tt.packets)
		})
	}
}

// A lone corrupt sync byte with the next period intact is repaired in place and
// delivered with a sync-byte event that lost nothing; it takes two in a row to
// declare a loss.
func TestSyncByteErrorKeepsPacket(t *testing.T) {
	var stream []byte
	stream = append(stream, syncPackets(3)...)
	hit := syncPacket()
	hit[0] = 0x00
	stream = append(stream, hit...)
	stream = append(stream, syncPackets(3)...)

	var got []RecoverableError
	cfg := PacketBufferConfig{SyncLock: true, OnRecover: func(e RecoverableError) { got = append(got, e) }}
	pb, err := NewPacketBuffer(context.Background(), bytes.NewReader(stream), cfg)
	require.NoError(t, err)
	p := NewPacket()
	var offsets []int64
	for {
		if err = pb.Next(p); err != nil {
			break
		}
		offsets = append(offsets, p.Offset)
		assert.Equal(t, syncByte, p.Raw()[0], "the delivered packet carries the restored sync byte")
	}
	require.ErrorIs(t, err, ErrNoMorePackets)
	assert.Equal(t, []int64{0, 188, 376, 564, 752, 940, 1128}, offsets, "the repaired packet is delivered in place")

	require.Len(t, got, 1)
	assert.Equal(t, ErrorKindSyncByte, got[0].Kind)
	assert.Equal(t, PIDUnset, got[0].PID)
	assert.Equal(t, int64(3*PacketSize), got[0].Offset)
	assert.Equal(t, int64(0), got[0].Dropped)
	assert.ErrorIs(t, got[0].Err, ErrPacketMustStartWithASyncByte)
}
