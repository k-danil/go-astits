package demux

import (
	"bytes"
	"encoding/binary"
	"sync"

	"github.com/k-danil/go-astits/v2/pes"
	"github.com/k-danil/go-astits/v2/psi"
	"github.com/k-danil/go-astits/v2/ts"
)

// PES is a complete parsed PES unit. The library owns the pool: an instance
// claimed via Demuxer.PES() stays valid across Next calls until Close.
type PES struct {
	Data              pes.Data
	AdaptationField   *ts.PacketAdaptationField
	PID               uint16
	ContinuityCounter uint8

	af  ts.PacketAdaptationField
	buf *dataPayload
}

var poolOfPES = sync.Pool{
	New: func() any {
		return &PES{}
	},
}

// Close returns the unit and its buffer to the pools. Idempotent.
func (d *PES) Close() {
	if d.buf == nil {
		return
	}
	poolOfPayload.put(d.buf)
	d.buf = nil
	d.Data = pes.Data{}
	d.AdaptationField = nil
	poolOfPES.Put(d)
}

// tableEvent is a pending table emission.
type tableEvent struct {
	data    psi.SectionSyntaxData
	pid     uint16
	ev      Event
	changed bool
}

// psiCache holds the last accepted section of a PID: the raw bytes for the
// repeat check and the emittable events reused on a repeat.
type psiCache struct {
	raw    []byte
	events []tableEvent
}

func tableEventKind(d psi.SectionSyntaxData) (ev Event, ok bool) {
	switch d.(type) {
	case *psi.PAT:
		return EventPAT, true
	case *psi.PMT:
		return EventPMT, true
	case *psi.NIT:
		return EventNIT, true
	case *psi.SDT:
		return EventSDT, true
	case *psi.TOT:
		return EventTOT, true
	case *psi.EIT:
		return EventEIT, true
	case *psi.TDT:
		return EventTDT, true
	}
	return 0, false
}

// processUnit parses a flushed unit: PSI updates the table state and queues
// EventTable emissions, PES materializes a pooled unit. Buffer ownership:
// PSI/garbage buffers return to the pool here, a PES buffer moves into the
// emitted unit.
func (dmx *Demuxer) processUnit(u unit) (emitted *PES, err error) {
	switch {
	case u.pid == ts.PIDCAT:
		poolOfPayload.put(u.buf)
	case u.isPSI:
		dmx.processPSI(u)
	case isPESPayload(u.buf.bs):
		d, _ := poolOfPES.Get().(*PES)
		d.PID = u.pid
		d.ContinuityCounter = u.cc
		d.buf = u.buf

		if perr := d.Data.Parse(u.buf.bs); perr != nil {
			d.Close()
			return nil, perr
		}

		if u.af != nil {
			d.af.CopyFrom(u.af)
			d.AdaptationField = &d.af
		} else {
			d.AdaptationField = nil
		}
		return d, nil
	default:
		// Unknown payload: no data will be produced
		poolOfPayload.put(u.buf)
	}
	return nil, nil
}

func (dmx *Demuxer) processPSI(u unit) {
	// PSI repeat dedup: an identical section carries no new information, so it
	// is not re-parsed. Without WithPSIRepeats it is not emitted either.
	if cache := dmx.psiPrev.Get(u.pid); cache != nil && bytes.Equal(cache.raw, u.buf.bs) {
		poolOfPayload.put(u.buf)
		if dmx.optPSIRepeats {
			for _, e := range cache.events {
				e.changed = false
				dmx.tblQueue = append(dmx.tblQueue, e)
			}
		}
		return
	}

	psiData, err := psi.Parse(u.buf.bs)
	if err != nil {
		// Swallow: a torn or corrupt section produces no emission
		poolOfPayload.put(u.buf)
		return
	}

	cache := dmx.psiPrev.GetOrAdd(u.pid)
	cache.raw = append(cache.raw[:0], u.buf.bs...)
	cache.events = cache.events[:0]
	poolOfPayload.put(u.buf)

	for _, s := range psiData.Sections {
		if s.Syntax == nil || s.Syntax.Data == nil {
			continue
		}
		ev, ok := tableEventKind(s.Syntax.Data)
		if !ok {
			continue
		}
		switch data := s.Syntax.Data.(type) {
		case *psi.PAT:
			dmx.pat = data
			for _, pgm := range data.Programs {
				// Program number 0 is reserved to NIT
				if pgm.ProgramNumber > 0 {
					dmx.programMap.Set(pgm.ProgramMapID, pgm.ProgramNumber)
				}
			}
		case *psi.PMT:
			dmx.pmt = data
		}
		e := tableEvent{pid: u.pid, data: s.Syntax.Data, ev: ev, changed: true}
		cache.events = append(cache.events, e)
		dmx.tblQueue = append(dmx.tblQueue, e)
	}
}

// isPESPayload checks whether the payload is a PES one
func isPESPayload(bs []byte) bool {
	if len(bs) < 4 {
		return false
	}
	return binary.BigEndian.Uint32(bs)>>8 == 1
}
