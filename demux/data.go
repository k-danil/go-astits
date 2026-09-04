package demux

import (
	"bytes"
	"encoding/binary"
	"errors"
	"sync"

	"github.com/k-danil/go-astits/v3/pes"
	"github.com/k-danil/go-astits/v3/psi"
	"github.com/k-danil/go-astits/v3/ts"
)

type PacketSpan struct {
	FirstPacketOffset int64
	LastPacketOffset  int64
	FirstPacketTag    uint64
	LastPacketTag     uint64
}

type PES struct {
	Data              pes.Data
	AdaptationField   *ts.PacketAdaptationField
	PID               uint16
	ContinuityCounter uint8
	// Truncated: the stream ended (EOF) before closing the unit, Data is what
	// arrived. Unbounded (length 0) may still be whole; bounded is short.
	Truncated bool
	PacketSpan

	af  ts.PacketAdaptationField
	buf *dataPayload
}

var poolOfPES = sync.Pool{
	New: func() any {
		return &PES{}
	},
}

// Close returns the unit and its buffer to the pools. Call it exactly once:
// a second call is a no-op only until the pool hands the instance to another
// unit, after which it would release that unit's buffer.
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

type tableEvent struct {
	sec     *psi.Section
	span    PacketSpan
	pid     uint16
	ev      Event
	changed bool
}

// A unit of nothing but damaged sections never replaces the cache, so a good unit returning after one still dedups.
type psiCache struct {
	raw    []byte
	events []tableEvent
	errs   []*psi.SectionError
}

func tableEventKind(d psi.SectionSyntaxData) (ev Event, ok bool) {
	switch d.(type) {
	case *psi.PAT:
		return EventPAT, true
	case *psi.PMT:
		return EventPMT, true
	case *psi.CAT:
		return EventCAT, true
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
	case *psi.BAT:
		return EventBAT, true
	case *psi.RST:
		return EventRST, true
	case *psi.DIT:
		return EventDIT, true
	case *psi.SIT:
		return EventSIT, true
	case *psi.ST:
		return EventST, true
	case *psi.TSDT:
		return EventTSDT, true
	}
	return 0, false
}

func (dmx *Demuxer) processUnit(u unit) (emitted *PES, err error) {
	switch {
	case u.headless:
		if dmx.optRecoverable {
			dmx.reportRecoverable(ts.RecoverableError{
				Kind: ts.ErrorKindTornUnit, PID: u.pid, Offset: u.LastPacketOffset,
				Dropped: int64(len(u.buf.bs)), Err: ts.ErrHeadlessUnit,
			})
		}
		poolOfPayload.put(u.buf)
	case u.isPSI:
		dmx.processPSI(u)
	case isPESPayload(u.buf.bs):
		d, _ := poolOfPES.Get().(*PES)
		d.PID = u.pid
		d.ContinuityCounter = u.cc
		d.buf = u.buf

		var perr error
		if u.truncated {
			perr = d.Data.ParseTruncated(u.buf.bs)
		} else {
			perr = d.Data.Parse(u.buf.bs)
		}
		if perr != nil {
			n := len(u.buf.bs)
			d.Close()
			if dmx.optRecoverable {
				dmx.reportRecoverable(ts.RecoverableError{
					Kind: ts.ErrorKindPES, PID: u.pid, Offset: u.LastPacketOffset, Dropped: int64(n), Err: perr,
				})
			}
			return nil, perr
		}
		d.Truncated = u.truncated && pesTruncated(&d.Data, len(u.buf.bs))
		d.PacketSpan = u.PacketSpan
		if dmx.optRecoverable && d.Data.Header.PacketLength == 0 && !pes.AllowsUnboundedLength(d.Data.Header.StreamID) {
			dmx.reportRecoverable(ts.RecoverableError{
				Kind: ts.ErrorKindPES, PID: u.pid, Offset: u.LastPacketOffset, Err: pes.ErrUnboundedNonVideo,
			})
		}

		if u.af != nil {
			d.af.CopyFrom(u.af)
			d.AdaptationField = &d.af
		} else {
			d.AdaptationField = nil
		}
		return d, nil
	default:
		if dmx.optRecoverable {
			dmx.reportRecoverable(ts.RecoverableError{
				Kind: ts.ErrorKindUnknownUnit, PID: u.pid, Offset: u.LastPacketOffset,
				Dropped: int64(len(u.buf.bs)), Err: ts.ErrUnknownPayload,
			})
		}
		poolOfPayload.put(u.buf)
	}
	return nil, nil
}

func pesTruncated(d *pes.Data, n int) bool {
	return d.Header.PacketLength == 0 || pes.HeaderSize+int(d.Header.PacketLength) > n
}

func (dmx *Demuxer) reportPSIError(pid uint16, offset int64, dropped int, err error) {
	kind := ts.ErrorKindPSI
	if errors.Is(err, psi.ErrCRC32Mismatch) {
		kind = ts.ErrorKindCRC
	}
	dmx.reportRecoverable(ts.RecoverableError{
		Kind: kind, PID: pid, Offset: offset, Dropped: int64(dropped), Err: err,
	})
}

func (dmx *Demuxer) processPSI(u unit) {
	if cache := dmx.psiPrev.Get(u.pid); cache != nil && bytes.Equal(cache.raw, u.buf.bs) {
		poolOfPayload.put(u.buf)
		dmx.reportSectionErrors(u.pid, u.LastPacketOffset, cache.errs)
		if dmx.optPSIRepeats {
			for _, e := range cache.events {
				e.changed = false
				e.span = u.PacketSpan
				dmx.tblQueue = append(dmx.tblQueue, e)
			}
		}
		return
	}

	psiData, err := psi.Parse(u.buf.bs)
	if err != nil {
		if dmx.optRecoverable {
			dmx.reportPSIError(u.pid, u.LastPacketOffset, len(u.buf.bs), err)
		}
		poolOfPayload.put(u.buf)
		return
	}

	dmx.reportSectionErrors(u.pid, u.LastPacketOffset, psiData.Errors)
	if len(psiData.Sections) == 0 {
		poolOfPayload.put(u.buf)
		return
	}

	cache := dmx.psiPrev.GetOrAdd(u.pid)
	cache.raw = append(cache.raw[:0], u.buf.bs...)
	cache.events = cache.events[:0]
	cache.errs = append(cache.errs[:0], psiData.Errors...)
	poolOfPayload.put(u.buf)

	for idx := range psiData.Sections {
		s := &psiData.Sections[idx]
		if s.Syntax == nil || s.Syntax.Data == nil {
			continue
		}
		ev, ok := tableEventKind(s.Syntax.Data)
		if !ok {
			continue
		}
		switch data := s.Syntax.Data.(type) {
		case *psi.PAT:
			dmx.applyPAT(data, &s.Syntax.Header)
		case *psi.PMT:
			if s.Syntax.Header.CurrentNextIndicator {
				dmx.pmt = data
			}
		}
		e := tableEvent{pid: u.pid, sec: s, ev: ev, changed: true, span: u.PacketSpan}
		cache.events = append(cache.events, e)
		dmx.tblQueue = append(dmx.tblQueue, e)
	}
}

func (dmx *Demuxer) reportSectionErrors(pid uint16, offset int64, errs []*psi.SectionError) {
	if !dmx.optRecoverable {
		return
	}
	for _, e := range errs {
		dmx.reportPSIError(pid, offset, e.Len, e)
	}
}

// A PAT announced for later keeps its PMT PIDs apart, so their sections still parse as PSI without claiming a PID that currently carries a stream; a version bump on the PAT in effect drops both maps, freeing PIDs that may return as elementary streams.
func (dmx *Demuxer) applyPAT(pat *psi.PAT, h *psi.SectionSyntaxHeader) {
	target := &dmx.acc.nextMap
	if h.CurrentNextIndicator {
		dmx.pat = pat
		if !dmx.patSeen || h.VersionNumber != dmx.patVersion {
			dmx.programMap.Clear()
			dmx.acc.nextMap.Clear()
			dmx.patVersion = h.VersionNumber
			dmx.patSeen = true
		}
		target = &dmx.programMap
	}
	for _, pgm := range pat.Programs {
		// Program number 0 is reserved to NIT
		if pgm.ProgramNumber > 0 {
			target.Set(pgm.ProgramMapID, pgm.ProgramNumber)
		}
	}
}

func isPESPayload(bs []byte) bool {
	if len(bs) < 4 {
		return false
	}
	return binary.BigEndian.Uint32(bs)>>8 == 1
}
