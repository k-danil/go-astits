package demux

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/k-danil/go-astits/v3/internal/pidmap"
	"github.com/k-danil/go-astits/v3/ts"
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

type accStep struct {
	cc      uint8
	pusi    bool
	payload string
	di      bool
	tei     bool
}

func runAcc(t *testing.T, steps []accStep) (units []string, torn []ts.RecoverableError) {
	t.Helper()
	var a accumulator
	pm := pidmap.Map[uint16]{}
	a.init(&pm, false, func(e ts.RecoverableError) { torn = append(torn, e) }, defaultMaxPESUnit, defaultMaxPSIUnit)
	for _, st := range steps {
		p := accPacket(0x100, st.cc, st.pusi, []byte(st.payload))
		p.Header.TransportErrorIndicator = st.tei
		if st.di {
			p.Header.HasAdaptationField = true
			p.AdaptationField.DiscontinuityIndicator = true
		}
		for _, u := range a.add(p, nil) {
			units = append(units, string(u.buf.bs))
			poolOfPayload.put(u.buf)
		}
	}
	return
}

// Only the unit start the indicator announces is exempt from the counter
// checks; a run of indicator packets with a continuous counter is one unit.
func TestAccumulatorDiscontinuityIndicator(t *testing.T) {
	tests := []struct {
		name      string
		steps     []accStep
		wantUnits []string
		wantTorn  []ts.RecoverableError
	}{
		{
			name: "unit start flushes the finished unit across the jump",
			steps: []accStep{
				{cc: 0, pusi: true, payload: "abc"}, {cc: 1, payload: "def"},
				{cc: 9, pusi: true, payload: "x", di: true}, {cc: 10, pusi: true, payload: "z"},
			},
			wantUnits: []string{"abcdef", "x"},
		},
		{
			name: "mid-unit jump tears it",
			steps: []accStep{
				{cc: 0, pusi: true, payload: "abc"}, {cc: 1, payload: "def"},
				{cc: 9, payload: "x", di: true},
			},
			wantTorn: []ts.RecoverableError{{Kind: ts.ErrorKindTornUnit, PID: 0x100, Dropped: 6, Err: ts.ErrDiscontinuity}},
		},
		{
			name: "unit start with a repeated counter is new, not a duplicate",
			steps: []accStep{
				{cc: 0, pusi: true, payload: "abc"}, {cc: 1, payload: "def"},
				{cc: 1, pusi: true, payload: "x", di: true}, {cc: 2, pusi: true, payload: "z"},
			},
			wantUnits: []string{"abcdef", "x"},
		},
		{
			name: "a run of indicator packets with a continuous counter is one unit",
			steps: []accStep{
				{cc: 0, pusi: true, payload: "abc", di: true}, {cc: 1, payload: "def", di: true},
				{cc: 2, payload: "ghi", di: true}, {cc: 3, pusi: true, payload: "z"},
			},
			wantUnits: []string{"abcdefghi"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			units, torn := runAcc(t, tt.steps)
			assert.Equal(t, tt.wantUnits, units)
			assert.Equal(t, tt.wantTorn, torn)
		})
	}
}

func TestAccumulatorTransportError(t *testing.T) {
	tests := []struct {
		name      string
		steps     []accStep
		wantUnits []string
		wantTorn  []ts.RecoverableError
	}{
		{
			name: "mid-unit tears once; the next unit start with the same counter is not a duplicate",
			steps: []accStep{
				{cc: 6, pusi: true, payload: "abc"}, {cc: 7, payload: "bad", tei: true},
				{cc: 7, pusi: true, payload: "new"}, {cc: 8, pusi: true, payload: "z"},
			},
			wantUnits: []string{"new"},
			wantTorn:  []ts.RecoverableError{{Kind: ts.ErrorKindTornUnit, PID: 0x100, Dropped: 6, Err: ts.ErrTransportError}},
		},
		{
			name: "at a unit start delivers the finished unit and starts none",
			steps: []accStep{
				{cc: 0, pusi: true, payload: "abc"}, {cc: 1, payload: "def"},
				{cc: 2, pusi: true, payload: "bad", tei: true}, {cc: 3, payload: "ghi"},
				{cc: 4, pusi: true, payload: "z"},
			},
			wantUnits: []string{"abcdef", "ghi"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			units, torn := runAcc(t, tt.steps)
			assert.Equal(t, tt.wantUnits, units)
			assert.Equal(t, tt.wantTorn, torn)
		})
	}
}

func TestAccumulatorDrainAscendingPIDs(t *testing.T) {
	var a accumulator
	pm := pidmap.Map[uint16]{}
	a.init(&pm, false, nil, defaultMaxPESUnit, defaultMaxPSIUnit)

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
	var slot pidSlot
	pm := pidmap.Map[uint16]{}
	a.init(&pm, true, nil, defaultMaxPESUnit, defaultMaxPSIUnit)
	var pids []int
	for i := range 256 {
		if a.isPSIPID(&slot, uint16(i)) {
			pids = append(pids, i)
		}
	}
	assert.Equal(t, []int{0, 1, 2, 16, 17, 18, 19, 20, 30, 31}, pids)

	// CAT and TSDT are base tables, not DVB ones: they stand without the option and without a program map entry
	a.init(&pm, false, nil, defaultMaxPESUnit, defaultMaxPSIUnit)
	assert.False(t, a.isPSIPID(&slot, uint16(0x12)))
	assert.True(t, a.isPSIPID(&slot, ts.PIDPAT))
	assert.True(t, a.isPSIPID(&slot, ts.PIDCAT))
	assert.True(t, a.isPSIPID(&slot, ts.PIDTSDT))
}

func TestIsPSIPIDNextPAT(t *testing.T) {
	var a accumulator
	pm := pidmap.Map[uint16]{}
	a.init(&pm, false, nil, defaultMaxPESUnit, defaultMaxPSIUnit)
	a.nextMap.Set(0x100, 1)

	var fresh, live pidSlot
	live.sawPES = true
	assert.True(t, a.isPSIPID(&fresh, 0x100))
	assert.False(t, a.isPSIPID(&live, 0x100), "a PID carrying a stream is not announced away")

	pm.Set(0x100, 1)
	assert.True(t, a.isPSIPID(&live, 0x100), "the PAT in effect wins over what the PID carried")
}
