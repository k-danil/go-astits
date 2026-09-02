package demux

import (
	"bytes"
	"encoding/binary"
	"errors"
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
	// Truncated: the stream ended (EOF) before closing the unit, Data is what
	// arrived. Unbounded (length 0) may still be whole; bounded is short.
	Truncated bool
	// Offsets of the first and the last packet that carried the unit, as
	// Packet.Offset (the packet's first byte in the stream).
	FirstPacketOffset int64
	LastPacketOffset  int64

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

// tableEvent is a pending table emission.
type tableEvent struct {
	sec     *psi.Section
	pid     uint16
	ev      Event
	changed bool
}

// psiCache holds the last unit of a PID that yielded at least one usable
// section: the raw bytes for the repeat check, the emittable events reused on
// a repeat, and the section errors re-reported on a repeat (each occurrence
// of a damaged section is a damage event of its own). A unit of nothing but
// damaged sections never replaces it, so a good unit coming back after one
// still dedups as the repeat it is.
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

// processUnit parses a flushed unit: PSI updates the table state and queues
// EventTable emissions, PES materializes a pooled unit. Buffer ownership:
// PSI/garbage buffers return to the pool here, a PES buffer moves into the
// emitted unit.
func (dmx *Demuxer) processUnit(u unit) (emitted *PES, err error) {
	switch {
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
					Kind: ts.ErrorKindPES, PID: u.pid, Offset: dmx.pkt.Offset, Dropped: int64(n), Err: perr,
				})
			}
			return nil, perr
		}
		d.Truncated = u.truncated && pesTruncated(&d.Data, len(u.buf.bs))
		d.FirstPacketOffset, d.LastPacketOffset = u.firstOffset, u.lastOffset
		if dmx.optRecoverable && d.Data.Header.PacketLength == 0 && !unboundedAllowed(d.Data.Header.StreamID) {
			dmx.reportRecoverable(ts.RecoverableError{
				Kind: ts.ErrorKindPES, PID: u.pid, Offset: dmx.pkt.Offset, Err: pes.ErrUnboundedNonVideo,
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
				Kind: ts.ErrorKindUnknownUnit, PID: u.pid, Offset: dmx.pkt.Offset,
				Dropped: int64(len(u.buf.bs)), Err: ts.ErrUnknownPayload,
			})
		}
		poolOfPayload.put(u.buf)
	}
	return nil, nil
}

const (
	pesStreamIDVideoMask = 0xf0
	pesStreamIDVideo     = 0xe0
	pesStreamIDExtended  = 0xfd
)

// unboundedAllowed: §2.4.3.7 allows PES_packet_length 0 for video only; the
// extended id carries VC-1 and Dirac video too, indistinguishable from its
// other payloads without the PMT, so it is not reported.
func unboundedAllowed(id pes.StreamID) bool {
	return id&pesStreamIDVideoMask == pesStreamIDVideo || id == pesStreamIDExtended
}

// Unbounded counts as truncated: nothing but the next unit start confirms its end.
func pesTruncated(d *pes.Data, n int) bool {
	return d.Header.PacketLength == 0 || pes.HeaderSize+int(d.Header.PacketLength) > n
}

// reportPSIError splits out a CRC32 mismatch (TR 101 290 CRC_error) from other
// section damage.
func (dmx *Demuxer) reportPSIError(pid uint16, dropped int, err error) {
	kind := ts.ErrorKindPSI
	if errors.Is(err, psi.ErrCRC32Mismatch) {
		kind = ts.ErrorKindCRC
	}
	dmx.reportRecoverable(ts.RecoverableError{
		Kind: kind, PID: pid, Offset: dmx.pkt.Offset, Dropped: int64(dropped), Err: err,
	})
}

func (dmx *Demuxer) processPSI(u unit) {
	// PSI repeat dedup: an identical section carries no new information, so it
	// is not re-parsed. Without WithPSIRepeats it is not emitted either.
	if cache := dmx.psiPrev.Get(u.pid); cache != nil && bytes.Equal(cache.raw, u.buf.bs) {
		poolOfPayload.put(u.buf)
		dmx.reportSectionErrors(u.pid, cache.errs)
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
		if dmx.optRecoverable {
			dmx.reportPSIError(u.pid, len(u.buf.bs), err)
		}
		poolOfPayload.put(u.buf)
		return
	}

	dmx.reportSectionErrors(u.pid, psiData.Errors)
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
			// Announced for later (current_next_indicator 0): surfaced, not in effect
			if s.Syntax.Header.CurrentNextIndicator {
				dmx.pmt = data
			}
		}
		e := tableEvent{pid: u.pid, sec: s, ev: ev, changed: true}
		cache.events = append(cache.events, e)
		dmx.tblQueue = append(dmx.tblQueue, e)
	}
}

func (dmx *Demuxer) reportSectionErrors(pid uint16, errs []*psi.SectionError) {
	if !dmx.optRecoverable {
		return
	}
	for _, e := range errs {
		dmx.reportPSIError(pid, e.Len, e)
	}
}

// applyPAT keeps the program map answering two questions: which PIDs carry
// PSI, and which programs are in effect. A PAT announced for later
// (current_next_indicator 0) only adds its PMT PIDs, so their sections are
// parsed as PSI when they arrive; a PAT in effect that bumps the version
// rebuilds the map, dropping the PIDs the new layout freed — they may come
// back as elementary streams. Sections of one version add up in any order.
func (dmx *Demuxer) applyPAT(pat *psi.PAT, h *psi.SectionSyntaxHeader) {
	if h.CurrentNextIndicator {
		dmx.pat = pat
		if !dmx.patSeen || h.VersionNumber != dmx.patVersion {
			dmx.programMap.Clear()
			dmx.patVersion = h.VersionNumber
			dmx.patSeen = true
		}
	}
	for _, pgm := range pat.Programs {
		// Program number 0 is reserved to NIT
		if pgm.ProgramNumber > 0 {
			dmx.programMap.Set(pgm.ProgramMapID, pgm.ProgramNumber)
		}
	}
}

// isPESPayload checks whether the payload is a PES one
func isPESPayload(bs []byte) bool {
	if len(bs) < 4 {
		return false
	}
	return binary.BigEndian.Uint32(bs)>>8 == 1
}
