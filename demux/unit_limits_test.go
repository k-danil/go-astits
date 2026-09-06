package demux

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v3/internal/pidmap"
	"github.com/k-danil/go-astits/v3/pes"
	"github.com/k-danil/go-astits/v3/ts"
)

func newAcc(report func(ts.RecoverableError), maxPES, maxPSI int) (a *accumulator, pm *pidmap.Map[uint16]) {
	a = &accumulator{}
	pm = &pidmap.Map[uint16]{}
	a.init(pm, false, report, maxPES, maxPSI)
	return
}

// The limit is enforced where the buffer would grow and when the unit is
// handed over, never per packet: a unit under the pool class but over the
// limit is caught at the flush, a unit over both at the growth, and 0
// delivers nothing.
func TestAccumulatorUnitSizeLimit(t *testing.T) {
	big := bytes.Repeat([]byte{'x'}, 1000)
	tests := []struct {
		name     string
		limit    int
		payloads [][]byte
		dropped  int64 // torn at the offending packet or at the next unit start
	}{
		{"caught at flush", 8, [][]byte{[]byte("abcdef"), []byte("ghij")}, 10},
		{"caught at growth, offending packet included", 2500, [][]byte{big, big, big}, 3000},
		{"nothing delivered at 0", 0, [][]byte{[]byte("abc")}, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var torn []ts.RecoverableError
			a, _ := newAcc(func(e ts.RecoverableError) { torn = append(torn, e) }, tt.limit, defaultMaxPSIUnit)

			var units []unit
			for i, pl := range tt.payloads {
				p := accPacket(0x100, uint8(i), i == 0, pl)
				p.Offset = int64(i) * ts.PacketSize
				units = a.add(p, units[:0])
				assert.Empty(t, units)
			}
			next := accPacket(0x100, uint8(len(tt.payloads)), true, []byte("fresh"))
			units = a.add(next, units[:0])
			assert.Empty(t, units, "the torn unit is not delivered")

			require.Len(t, torn, 1)
			assert.Equal(t, ts.ErrorKindTornUnit, torn[0].Kind)
			require.ErrorIs(t, torn[0].Err, ts.ErrUnitTooLarge)
			assert.Equal(t, tt.dropped, torn[0].Dropped)
		})
	}
}

func TestAccumulatorPSICompletesAcrossThreePackets(t *testing.T) {
	a, _ := newAcc(nil, defaultMaxPESUnit, defaultMaxPSIUnit)
	b := psiBytes()
	units := a.add(accPacket(ts.PIDPAT, 0, true, b[:60]), nil)
	assert.Empty(t, units)
	units = a.add(accPacket(ts.PIDPAT, 1, false, b[60:120]), units[:0])
	assert.Empty(t, units)
	units = a.add(accPacket(ts.PIDPAT, 2, false, b[120:]), units[:0])
	require.Len(t, units, 1)
	assert.Equal(t, b, units[0].buf.bs)
	poolOfPayload.put(units[0].buf)
}

// The resumable scan belongs to one unit: a shorter unit following a longer
// one on the same PID must complete on its own length.
func TestAccumulatorPSIScanRestartsPerUnit(t *testing.T) {
	a, _ := newAcc(nil, defaultMaxPESUnit, defaultMaxPSIUnit)
	long := psiBytes()
	short := append([]byte{0x00}, patSection(0, true, 0x100)...)
	require.Less(t, len(short), len(long))

	units := a.add(accPacket(ts.PIDPAT, 0, true, long), nil)
	require.Len(t, units, 1)
	poolOfPayload.put(units[0].buf)
	units = a.add(accPacket(ts.PIDPAT, 1, true, short), units[:0])
	require.Len(t, units, 1, "completes without waiting for the next unit start")
	assert.Equal(t, short, units[0].buf.bs)
	poolOfPayload.put(units[0].buf)
}

// §2.4.3.3: a repeat is accepted twice at most, must be byte-identical, and
// is never appended.
func TestAccumulatorRepeats(t *testing.T) {
	t.Run("third repeat is a gap and stays dropped", func(t *testing.T) {
		var events []ts.RecoverableError
		a, _ := newAcc(func(e ts.RecoverableError) { events = append(events, e) }, defaultMaxPESUnit, defaultMaxPSIUnit)
		var units []unit
		for _, p := range []*ts.Packet{
			accPacket(0x100, 0, true, []byte("abc")),
			accPacket(0x100, 1, false, []byte("def")),
			accPacket(0x100, 1, false, []byte("def")),
			accPacket(0x100, 1, false, []byte("def")),
			accPacket(0x100, 1, false, []byte("def")),
		} {
			units = a.add(p, units[:0])
			assert.Empty(t, units)
		}
		require.Len(t, events, 1, "the fourth repeat adds no event")
		assert.Equal(t, ts.ErrorKindTornUnit, events[0].Kind)
		require.ErrorIs(t, events[0].Err, ts.ErrContinuityGap)
		assert.Equal(t, int64(6), events[0].Dropped)

		units = a.add(accPacket(0x100, 2, true, []byte("next")), units[:0])
		assert.Empty(t, units, "no unit is made of the repeats")
		units = a.add(accPacket(0x100, 3, true, []byte("after")), units[:0])
		require.Len(t, units, 1)
		assert.Equal(t, []byte("next"), units[0].buf.bs)
		poolOfPayload.put(units[0].buf)
	})

	// a completed section leaves no tail to compare against: its repeat is still a repeat
	t.Run("a repeated PSI packet is dropped after its section completed", func(t *testing.T) {
		var events []ts.RecoverableError
		a, _ := newAcc(func(e ts.RecoverableError) { events = append(events, e) }, defaultMaxPESUnit, defaultMaxPSIUnit)
		sec := append([]byte{0x00}, patSection(0, true, 0x100)...)
		var delivered int
		var units []unit
		for range 2 {
			units = a.add(accPacket(ts.PIDPAT, 5, true, sec), units[:0])
			delivered += len(units)
			for _, u := range units {
				poolOfPayload.put(u.buf)
			}
		}
		assert.Equal(t, 1, delivered, "the duplicate delivers no second section")
		assert.Empty(t, events)
	})

	t.Run("a repeat after ordinary packets is a repeat again", func(t *testing.T) {
		var events []ts.RecoverableError
		a, _ := newAcc(func(e ts.RecoverableError) { events = append(events, e) }, defaultMaxPESUnit, defaultMaxPSIUnit)
		var units []unit
		for _, p := range []*ts.Packet{
			accPacket(0x100, 0, true, []byte("abc")),
			accPacket(0x100, 1, false, []byte("def")),
			accPacket(0x100, 1, false, []byte("def")),
			accPacket(0x100, 2, false, []byte("ghi")),
			accPacket(0x100, 3, false, []byte("jkl")),
			accPacket(0x100, 3, false, []byte("jkl")),
		} {
			units = a.add(p, units[:0])
			assert.Empty(t, units)
		}
		assert.Empty(t, events)
		units = a.add(accPacket(0x100, 4, true, []byte("next")), units[:0])
		require.Len(t, units, 1)
		assert.Equal(t, []byte("abcdefghijkl"), units[0].buf.bs)
		poolOfPayload.put(units[0].buf)
	})

	t.Run("differing repeat is reported and not appended", func(t *testing.T) {
		var events []ts.RecoverableError
		a, _ := newAcc(func(e ts.RecoverableError) { events = append(events, e) }, defaultMaxPESUnit, defaultMaxPSIUnit)
		var units []unit
		for _, p := range []*ts.Packet{
			accPacket(0x100, 0, true, []byte("abc")),
			accPacket(0x100, 1, false, []byte("def")),
			accPacket(0x100, 1, false, []byte("dXf")),
		} {
			units = a.add(p, units[:0])
			assert.Empty(t, units)
		}
		require.Len(t, events, 1)
		assert.Equal(t, ts.ErrorKindPacketDrop, events[0].Kind)
		require.ErrorIs(t, events[0].Err, ts.ErrDuplicateMismatch)
		assert.Equal(t, int64(3), events[0].Dropped)

		units = a.add(accPacket(0x100, 2, true, []byte("next")), units[:0])
		require.Len(t, units, 1)
		assert.Equal(t, []byte("abcdef"), units[0].buf.bs)
		poolOfPayload.put(units[0].buf)
	})
}

func TestDemuxerPacketCountsAndNullPackets(t *testing.T) {
	stream := payloadPacket(0x100, 0, true, unboundedPES)
	for cc := range 3 {
		stream = append(stream, payloadPacket(ts.PIDNull, uint8(cc), false, bytes.Repeat([]byte{0xff}, 184))...)
	}
	stream = append(stream, payloadPacket(0x100, 1, true, unboundedPES)...)
	dmx := New(context.Background(), bytes.NewReader(stream), WithPacketSize(ts.PacketSize), WithRecoverableErrors())
	defer dmx.Close()

	var pesUnits int
	for ev, err := range dmx.Events() {
		if re, ok := errors.AsType[*ts.RecoverableError](err); ok {
			assert.NotEqual(t, ts.PIDNull, re.PID, "no event on the null PID: %v", re)
			continue
		}
		require.NoError(t, err)
		if ev == EventPES {
			pesUnits++
		}
	}
	assert.Equal(t, 2, pesUnits)
	assert.Equal(t, map[uint16]uint64{0x100: 2, ts.PIDNull: 3}, dmx.PacketCounts())
}

// A non-video stream may not leave PES_packet_length at 0 (§2.4.3.7): the
// unit is still delivered, the violation is reported with nothing dropped.
// Any video stream_id (0xE0-0xEF) and the extended id are left alone.
func TestDemuxerUnboundedNonVideoReported(t *testing.T) {
	tests := []struct {
		name     string
		streamID byte
		reported bool
	}{
		{"audio", 0xc0, true},
		{"private stream 1", 0xbd, true},
		{"video 1", 0xe1, false},
		{"extended", 0xfd, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unit := []byte{0x00, 0x00, 0x01, tt.streamID, 0x00, 0x00, 0x80, 0x00, 0x00, 'e', 's'}
			stream := payloadPacket(0x101, 0, true, unit)
			stream = append(stream, payloadPacket(0x101, 1, true, unit)...)
			dmx := New(context.Background(), bytes.NewReader(stream), WithPacketSize(ts.PacketSize), WithRecoverableErrors())
			defer dmx.Close()

			ev, err := dmx.Next()
			require.NoError(t, err)
			require.Equal(t, EventPES, ev)
			assert.True(t, bytes.HasPrefix(dmx.PES().Data.Data, []byte("es")), "the unit is delivered")

			ev, err = dmx.Next()
			if !tt.reported {
				require.NoError(t, err)
				assert.Equal(t, EventPES, ev, "the drained second unit, no error in between")
				return
			}
			require.Error(t, err)
			assert.Equal(t, EventError, ev)
			var re *ts.RecoverableError
			require.ErrorAs(t, err, &re)
			assert.Equal(t, ts.ErrorKindPES, re.Kind)
			assert.Equal(t, uint16(0x101), re.PID)
			assert.Equal(t, int64(0), re.Dropped)
			assert.ErrorIs(t, re.Err, pes.ErrUnboundedNonVideo)
		})
	}
}
