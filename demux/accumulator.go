package demux

import (
	"bytes"
	"encoding/binary"

	"github.com/k-danil/go-astits/v2/internal/pidmap"
	"github.com/k-danil/go-astits/v2/psi"
	"github.com/k-danil/go-astits/v2/ts"
)

// Accumulator buffer sizing: exact hints from the unit's first bytes where the
// format provides them, otherwise the slot's sticky-max class with a floor.
// The 64K floor for unbounded PES (video) absorbs cold-start growth: measured
// on a 1-hour SD stream it leaves 1.7% of bytes to growth copies.
const (
	unboundedPESFloorClass = 6 // 64 KB
	defaultFloorClass      = 1 // 2 KB
)

// pidSlot accumulates one payload unit of a PID into a contiguous buffer:
// packets are one-shot scratch, their payloads are copied out immediately.
type pidSlot struct {
	buf *dataPayload

	// Two AF storages toggled per unit: the flushed unit's pointer must stay
	// intact while the next unit's start overwrites the other one.
	af    [2]ts.PacketAdaptationField
	afIdx uint8
	hasAF bool
	cc    uint8 // CC of the unit's first packet

	lastCC     uint8
	seenPacket bool
	lastWasDup bool
	lastLen    int // payload length of the last packet appended, 0 once the unit is gone

	firstOffset int64 // packet that started the unit
	lastOffset  int64 // last packet that fed it

	psiScan int // psiComplete resumes here: the next section header to inspect

	sticky  uint8 // sticky-max size class over the slot's lifetime
	started bool
	isPSI   bool
	packets uint64
}

// accumulator replaces the per-PID packet lists: it owns per-PID slots and
// flushes completed units as contiguous buffers.
type accumulator struct {
	slots      pidmap.Map[pidSlot]
	programMap *pidmap.Map[uint16]

	// One-entry slot cache: a run of packets on one PID (the common case on a
	// single-program stream) skips the hashed lookup. Invalidated whenever
	// slots may reallocate.
	lastPID   uint16
	lastSlot  *pidSlot
	report    func(ts.RecoverableError) // nil keeps the silent fast path
	maxPES    int                       // unit size limits on the WithMaxUnitSize scale
	maxPSI    int
	dvbTables bool

	keysArr [packetPoolPreallocPIDs]uint16
	valsArr [packetPoolPreallocPIDs]pidSlot
}

const packetPoolPreallocPIDs = 8

func (a *accumulator) init(programMap *pidmap.Map[uint16], dvbTables bool, report func(ts.RecoverableError), maxPES, maxPSI int) {
	a.slots = pidmap.Map[pidSlot]{Keys: a.keysArr[:0], Vals: a.valsArr[:0]}
	a.lastSlot = nil
	a.programMap = programMap
	a.report = report
	a.maxPES = maxPES
	a.maxPSI = maxPSI
	a.dvbTables = dvbTables
}

// unit is a flushed payload unit handed to the parse stage. buf ownership
// moves to the receiver. truncated marks an EOF drain: the stream never
// closed the unit, so its end is not confirmed.
type unit struct {
	buf         *dataPayload
	af          *ts.PacketAdaptationField
	firstOffset int64
	lastOffset  int64
	cc          uint8
	pid         uint16
	isPSI       bool
	truncated   bool
}

func (a *accumulator) isPSIPID(pid uint16) bool {
	return pid == ts.PIDPAT ||
		a.programMap.Has(pid) ||
		(a.dvbTables && (pid == ts.PIDCAT || pid == ts.PIDTSDT || (pid >= 0x10 && pid <= 0x14) || (pid >= 0x1e && pid <= 0x1f)))
}

// add consumes the packet's payload and appends completed units (zero, one,
// or — for a torn PSI flushed by the same packet that completes the next
// section — two) to out. Buffer ownership moves with the units.
func (a *accumulator) add(p *ts.Packet, out []unit) []unit {
	slot := a.lastSlot
	if slot == nil || p.Header.PID != a.lastPID {
		if slot = a.slots.Get(p.Header.PID); slot == nil {
			slot = a.slots.GetOrAdd(p.Header.PID)
		}
		a.lastPID, a.lastSlot = p.Header.PID, slot
	}
	slot.packets++

	// A null packet's payload and counter are undefined (§2.4.3.3): counted,
	// never accumulated
	if !p.Header.HasPayload || p.Header.PID == ts.PIDNull {
		return out
	}

	if p.Header.TransportErrorIndicator || p.Header.TransportScramblingControl != ts.ScramblingControlNotScrambled {
		// The unit start still closes the previous unit; the unusable payload
		// itself opens none
		if !p.Header.PayloadUnitStartIndicator {
			reason := ts.ErrScrambled
			if p.Header.TransportErrorIndicator {
				reason = ts.ErrTransportError
			}
			a.tear(slot, p.Header.PID, p.Offset, reason, 0)
			return out
		}
		if slot.started {
			if u, ok := a.finish(slot, p.Header.PID, p.Offset); ok {
				out = append(out, u)
			}
		}
		slot.seenPacket = false
		return out
	}

	// §2.4.3.4 lets the indicator stay set on every PCR-PID packet until the
	// next PCR, so only the unit start it announces is exempt from the counter
	// checks; a mid-unit indicator packet with a continuous counter is payload
	discontinuity := p.Header.HasAdaptationField && p.AdaptationField.DiscontinuityIndicator
	jumpAllowed := discontinuity && p.Header.PayloadUnitStartIndicator
	if !jumpAllowed && slot.seenPacket && p.Header.ContinuityCounter == slot.lastCC {
		// §2.4.3.3: two and only two, byte-identical. A third repeat is a
		// counter discontinuity, and no repeat is ever appended.
		if !slot.lastWasDup {
			slot.lastWasDup = true
			a.checkDuplicate(slot, p)
		} else {
			a.tear(slot, p.Header.PID, p.Offset, ts.ErrContinuityGap, 0)
			slot.seenPacket = true // the counter stays known: further repeats are still repeats
		}
		return out
	}
	slot.lastWasDup = false
	if slot.started && !jumpAllowed && p.Header.ContinuityCounter != (slot.lastCC+1)%16 {
		reason := ts.ErrContinuityGap
		if discontinuity {
			reason = ts.ErrDiscontinuity
		}
		a.tear(slot, p.Header.PID, p.Offset, reason, 0)
	}

	if p.Header.PayloadUnitStartIndicator {
		if slot.started {
			if u, ok := a.finish(slot, p.Header.PID, p.Offset); ok {
				out = append(out, u)
			}
		}
		slot.start(p, a.isPSIPID(p.Header.PID))
	} else if !slot.started {
		// A headless prefix (stream picked up mid-unit) accumulates too and
		// flushes on the next PayloadUnitStartIndicator, matching the packet
		// list behavior; the parse stage rejects it if it is garbage.
		slot.start(p, a.isPSIPID(p.Header.PID))
	}
	// After finish/start: a tear inside finish clears seenPacket, and the
	// unit this packet opens must still see its own repeat
	slot.lastCC = p.Header.ContinuityCounter
	slot.seenPacket = true

	if need := len(slot.buf.bs) + len(p.Payload); need > cap(slot.buf.bs) {
		if overLimit(need, a.limitFor(slot)) {
			a.tear(slot, p.Header.PID, p.Offset, ts.ErrUnitTooLarge, len(p.Payload))
			return out
		}
		slot.grow(need)
	}
	slot.buf.bs = append(slot.buf.bs, p.Payload...)
	slot.lastLen = len(p.Payload)
	slot.lastOffset = p.Offset

	// A PSI unit completes by section lengths, without waiting for the next
	// PayloadUnitStartIndicator
	if slot.isPSI && slot.psiComplete() {
		if u, ok := a.finish(slot, p.Header.PID, p.Offset); ok {
			out = append(out, u)
		}
	}
	return out
}

// checkDuplicate compares a repeated packet with the last payload appended,
// while that payload is still the tail of the unit; without it the repeat is
// taken on the counter alone.
func (a *accumulator) checkDuplicate(slot *pidSlot, p *ts.Packet) {
	if a.report == nil || !slot.started || slot.lastLen == 0 || slot.lastLen > len(slot.buf.bs) {
		return
	}
	if !bytes.Equal(p.Payload, slot.buf.bs[len(slot.buf.bs)-slot.lastLen:]) {
		a.report(ts.RecoverableError{
			Kind: ts.ErrorKindPacketDrop, PID: p.Header.PID, Offset: p.Offset,
			Dropped: int64(len(p.Payload)), Err: ts.ErrDuplicateMismatch,
		})
	}
}

func (a *accumulator) limitFor(slot *pidSlot) int {
	if slot.isPSI {
		return a.maxPSI
	}
	return a.maxPES
}

// overLimit: -1 is never over.
func overLimit(size, limit int) bool {
	return limit >= 0 && size > limit
}

func (a *accumulator) finish(slot *pidSlot, pid uint16, offset int64) (u unit, ok bool) {
	if !slot.started {
		return
	}
	if overLimit(len(slot.buf.bs), a.limitFor(slot)) {
		a.tear(slot, pid, offset, ts.ErrUnitTooLarge, 0)
		return
	}
	return slot.flush(pid)
}

// tear drops the slot's unit; lost is what the offending packet carried on
// top of the buffer, so Dropped covers everything the unit cost.
func (a *accumulator) tear(slot *pidSlot, pid uint16, offset int64, reason error, lost int) {
	if !slot.started {
		return
	}
	if a.report != nil && len(slot.buf.bs)+lost > 0 {
		a.report(ts.RecoverableError{
			Kind: ts.ErrorKindTornUnit, PID: pid, Offset: offset,
			Dropped: int64(len(slot.buf.bs) + lost), Err: reason,
		})
	}
	slot.release()
	slot.seenPacket = false
}

// start begins a new unit from a PayloadUnitStartIndicator packet.
func (s *pidSlot) start(p *ts.Packet, isPSI bool) {
	s.started = true
	s.isPSI = isPSI
	s.cc = p.Header.ContinuityCounter
	s.firstOffset = p.Offset
	s.psiScan = 0
	if p.Header.HasAdaptationField {
		s.afIdx ^= 1
		s.af[s.afIdx].CopyFrom(&p.AdaptationField)
		s.hasAF = true
	} else {
		s.hasAF = false
	}

	if s.buf == nil {
		s.buf = poolOfPayload.getClass(s.classFor(p.Payload, isPSI))
	}
	s.buf.bs = s.buf.bs[:0]
}

// classFor picks the starting size class: exact hint when the first bytes
// carry the unit length, sticky-max with a floor otherwise.
func (s *pidSlot) classFor(payload []byte, isPSI bool) uint8 {
	if isPSI {
		return maxClass(s.sticky, defaultFloorClass)
	}
	if len(payload) >= 6 && payload[0] == 0 && payload[1] == 0 && payload[2] == 1 {
		if pl := binary.BigEndian.Uint16(payload[4:6]); pl > 0 {
			return maxClass(classOf(int(pl)+6), s.sticky)
		}
		// Unbounded PES is video: start high to absorb cold-start growth
		return maxClass(s.sticky, unboundedPESFloorClass)
	}
	return maxClass(s.sticky, defaultFloorClass)
}

func (s *pidSlot) grow(need int) {
	grown := poolOfPayload.getClass(classOf(need))
	grown.bs = grown.bs[:len(s.buf.bs)]
	copy(grown.bs, s.buf.bs)
	poolOfPayload.put(s.buf)
	s.buf = grown
}

// flush hands the accumulated unit over; the slot remembers the size class
// for the next unit of this PID.
func (s *pidSlot) flush(pid uint16) (u unit, ok bool) {
	if !s.started || len(s.buf.bs) == 0 {
		s.release()
		return
	}
	s.sticky = maxClass(s.sticky, classOf(len(s.buf.bs)))
	u = unit{buf: s.buf, cc: s.cc, pid: pid, isPSI: s.isPSI, firstOffset: s.firstOffset, lastOffset: s.lastOffset}
	if s.hasAF {
		u.af = &s.af[s.afIdx]
	}
	s.buf = nil
	s.started = false
	s.lastLen = 0
	return u, true
}

func (s *pidSlot) release() {
	if s.buf != nil {
		poolOfPayload.put(s.buf)
		s.buf = nil
	}
	s.started = false
	s.lastLen = 0
}

// psiSectionHeaderLen is table_id plus the 16 bits holding section_length.
const psiSectionHeaderLen = 3

// psiComplete reports whether the accumulated buffer already holds all its
// sections: a scan over lengths, resumed from where the last call stopped so
// a unit costs one pass however many packets feed it.
func (s *pidSlot) psiComplete() bool {
	bs := s.buf.bs
	if s.psiScan == 0 {
		if len(bs) == 0 {
			return false
		}
		s.psiScan = 1 + int(bs[0]) // pointer filler bytes
	}
	for s.psiScan < len(bs) {
		if psi.TableID(bs[s.psiScan]).StopsParsing() {
			return true
		}
		if s.psiScan+psiSectionHeaderLen > len(bs) {
			return false
		}
		s.psiScan += psiSectionHeaderLen + int(binary.BigEndian.Uint16(bs[s.psiScan+1:])&0x0fff)
	}
	return s.psiScan == len(bs)
}

// drain flushes the unfinished unit of the lowest PID that has one: EOF tails
// come out in ascending PID order. A tail over its size limit is torn and the
// next PID is tried, so one oversized tail does not hide the others.
func (a *accumulator) drain() (u unit, ok bool) {
	for {
		minIdx := -1
		for i := range a.slots.Vals {
			if !a.slots.Vals[i].started || len(a.slots.Vals[i].buf.bs) == 0 {
				continue
			}
			if minIdx < 0 || a.slots.Keys[i] < a.slots.Keys[minIdx] {
				minIdx = i
			}
		}
		if minIdx < 0 {
			return
		}
		slot := &a.slots.Vals[minIdx]
		if u, ok = a.finish(slot, a.slots.Keys[minIdx], slot.lastOffset); ok {
			u.truncated = true
			return
		}
	}
}

// close releases every slot buffer.
func (a *accumulator) close() {
	for i := range a.slots.Vals {
		a.slots.Vals[i].release()
	}
}

func classOf(size int) uint8 {
	var c uint8
	for s := 1 << 10; s < size; s <<= 1 {
		c++
	}
	return c
}

func maxClass(a, b uint8) uint8 {
	if a > b {
		return a
	}
	return b
}
