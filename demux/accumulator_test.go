package demux

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v2/internal/pidmap"
	"github.com/k-danil/go-astits/v2/ts"
)

func accPacket(pid uint16, cc uint8, pusi bool, payload []byte) *ts.Packet {
	return &ts.Packet{
		Header: ts.PacketHeader{
			PID:                       pid,
			ContinuityCounter:         cc,
			HasPayload:                true,
			PayloadUnitStartIndicator: pusi,
		},
		Payload: payload,
	}
}

func TestAccumulatorFlushOnUnitStart(t *testing.T) {
	var a accumulator
	pm := pidmap.Map[uint16]{}
	a.init(&pm, false)

	var units []unit
	units = a.add(accPacket(1, 0, true, []byte("abc")), units[:0])
	assert.Empty(t, units)
	units = a.add(accPacket(1, 1, false, []byte("def")), units[:0])
	assert.Empty(t, units)
	units = a.add(accPacket(1, 2, true, []byte("next")), units[:0])
	require.Len(t, units, 1)
	assert.Equal(t, []byte("abcdef"), units[0].buf.bs)
	assert.Equal(t, uint16(1), units[0].pid)
	assert.Equal(t, uint8(0), units[0].cc)

	// same CC with payload repeated: the duplicate is dropped
	units = a.add(accPacket(1, 2, true, []byte("next")), units[:0])
	assert.Empty(t, units)

	// discontinuity (CC jump) drops the unfinished unit; the offending packet
	// starts a headless unit that flushes on the next unit start — the parse
	// stage is the one to reject it
	units = a.add(accPacket(1, 5, false, []byte("torn")), units[:0])
	assert.Empty(t, units)
	units = a.add(accPacket(1, 6, true, []byte("fresh")), units[:0])
	require.Len(t, units, 1)
	assert.Equal(t, []byte("torn"), units[0].buf.bs)
	assert.Equal(t, uint8(5), units[0].cc)
}

func TestAccumulatorPSICompletes(t *testing.T) {
	var a accumulator
	pm := pidmap.Map[uint16]{}
	a.init(&pm, false)

	// PAT PID with a complete single section: flushes without waiting for
	// the next unit start
	b := psiBytes()
	units := a.add(accPacket(ts.PIDPAT, 0, true, b[:147]), nil)
	assert.Empty(t, units)
	units = a.add(accPacket(ts.PIDPAT, 1, false, b[147:]), units[:0])
	require.Len(t, units, 1)
	assert.True(t, units[0].isPSI)
	assert.Equal(t, b, units[0].buf.bs)
}

func TestAccumulatorDrainAscendingPIDs(t *testing.T) {
	var a accumulator
	pm := pidmap.Map[uint16]{}
	a.init(&pm, false)

	_ = a.add(accPacket(0x300, 0, true, []byte("high")), nil)
	_ = a.add(accPacket(0x100, 0, true, []byte("low")), nil)
	_ = a.add(accPacket(0x200, 0, true, []byte("mid")), nil)

	var pids []uint16
	for {
		u, ok := a.drain()
		if !ok {
			break
		}
		pids = append(pids, u.pid)
		poolOfPayload.put(u.buf)
	}
	assert.Equal(t, []uint16{0x100, 0x200, 0x300}, pids)
}

func TestIsPSIPID(t *testing.T) {
	var a accumulator
	pm := pidmap.Map[uint16]{}
	a.init(&pm, true)
	var pids []int
	for i := 0; i <= 255; i++ {
		if a.isPSIPID(uint16(i)) {
			pids = append(pids, i)
		}
	}
	assert.Equal(t, []int{0, 1, 16, 17, 18, 19, 20, 30, 31}, pids)
	pm.Set(uint16(1), uint16(0))
	assert.True(t, a.isPSIPID(uint16(1)))

	// DVB ranges are ignored without the option
	a.init(&pm, false)
	assert.False(t, a.isPSIPID(uint16(0x12)))
	assert.True(t, a.isPSIPID(ts.PIDPAT))
	assert.True(t, a.isPSIPID(uint16(1)))
}

func TestIsPESPayload(t *testing.T) {
	assert.False(t, isPESPayload([]byte{0, 0, 0}))
	assert.False(t, isPESPayload([]byte{0, 0, 0, 1}))
	assert.True(t, isPESPayload([]byte{0, 0, 1, 0xc0}))
}
