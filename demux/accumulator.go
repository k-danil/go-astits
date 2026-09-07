package demux

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math/bits"

	"github.com/k-danil/go-astits/v3/internal/pidmap"
	"github.com/k-danil/go-astits/v3/pes"
	"github.com/k-danil/go-astits/v3/psi"
	"github.com/k-danil/go-astits/v3/ts"
)

const (
	unboundedPESFloorClass = 6
	defaultFloorClass      = 1
	pesLengthOffset        = 4
)

// Packets are one-shot scratch: their payloads must be copied out, never retained.
type pidSlot struct {
	buf *dataPayload

	// Two AF storages toggled per unit: the flushed unit's pointer must stay
	// intact while the next unit's start overwrites the other one.
	af      [2]ts.PacketAdaptationField
	afIdx   uint8
	hasAF   bool
	firstCC uint8

	lastCC     uint8
	seenPacket bool
	lastWasDup bool
	lastLen    int

	firstOffset int64
	lastOffset  int64
	firstTag    uint64
	lastTag     uint64

	psiScan int

	sticky   uint8
	started  bool
	headless bool
	isPSI    bool
	sawPES   bool
	packets  uint64
}

type accumulator struct {
	slots      pidmap.Map[pidSlot]
	programMap *pidmap.Map[uint16]
	nextMap    pidmap.Map[uint16]

	// lastSlot points into slots.Vals: reassign it after any GetOrAdd, which may reallocate.
	lastPID   uint16
	lastSlot  *pidSlot
	report    func(ts.RecoverableError)
	maxPES    int
	maxPSI    int
	dvbTables bool

	keysArr [packetPoolPreallocPIDs]uint16
	valsArr [packetPoolPreallocPIDs]pidSlot
}

const packetPoolPreallocPIDs = 8

func (a *accumulator) init(programMap *pidmap.Map[uint16], dvbTables bool, report func(ts.RecoverableError), maxPES, maxPSI int) {
	a.slots = pidmap.Map[pidSlot]{Keys: a.keysArr[:0], Vals: a.valsArr[:0]}
	a.nextMap = pidmap.Map[uint16]{}
	a.lastSlot = nil
	a.programMap = programMap
	a.report = report
	a.maxPES = maxPES
	a.maxPSI = maxPSI
	a.dvbTables = dvbTables
}

type unit struct {
	buf *dataPayload
	af  *ts.PacketAdaptationField
	PacketSpan
	cc        uint8
	pid       uint16
	isPSI     bool
	headless  bool
	truncated bool
}

const (
	dvbSIFirstPID = 0x10
	dvbSILastPID  = 0x14
	dvbDITPID     = 0x1e
	dvbSITPID     = 0x1f
)

// An announced PAT (nextMap) may not claim a PID that already delivered elementary-stream units.
func (a *accumulator) isPSIPID(slot *pidSlot, pid uint16) bool {
	return pid <= ts.PIDTSDT ||
		a.programMap.Has(pid) ||
		(a.dvbTables && ((pid >= dvbSIFirstPID && pid <= dvbSILastPID) || pid == dvbDITPID || pid == dvbSITPID)) ||
		(!slot.sawPES && a.nextMap.Has(pid))
}

func (a *accumulator) startUnit(slot *pidSlot, p *ts.Packet) {
	isPSI := a.isPSIPID(slot, p.Header.PID)
	slot.start(p, isPSI, a.limitOf(isPSI))
}

// One packet can yield two units: it closes the open one and can complete the next in the same packet.
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
		if !p.Header.PayloadUnitStartIndicator {
			reason := ts.ErrScrambled
			if p.Header.TransportErrorIndicator {
				reason = ts.ErrTransportError
			}
			return a.tear(slot, p.Header.PID, p.Offset, reason, len(p.Payload), out)
		}
		if slot.started {
			if u, ok := a.finish(slot, p.Header.PID, p.Offset); ok {
				out = append(out, u)
			}
		}
		slot.seenPacket = false
		return out
	}

	// §2.4.3.4: the indicator may stay set on every PCR-PID packet, so only the unit start it announces skips the counter checks.
	discontinuity := p.Header.HasAdaptationField && p.AdaptationField.DiscontinuityIndicator
	jumpAllowed := discontinuity && p.Header.PayloadUnitStartIndicator
	if !jumpAllowed && slot.seenPacket && p.Header.ContinuityCounter == slot.lastCC {
		if !p.Header.PayloadUnitStartIndicator || !slot.differsFromTail(p.Payload) {
			// §2.4.3.3: one repeat at most; a further one is a counter discontinuity
			if !slot.lastWasDup {
				slot.lastWasDup = true
				a.checkDuplicate(slot, p)
			} else {
				out = a.tear(slot, p.Header.PID, p.Offset, ts.ErrContinuityGap, 0, out)
				slot.seenPacket = true // the counter stays known: further repeats are still repeats
			}
			return out
		}
		// the fall-through would tear with ErrContinuityGap; the mismatch is the one counter break a consumer's own check cannot see
		out = a.tear(slot, p.Header.PID, p.Offset, ts.ErrDuplicateMismatch, 0, out)
	}
	slot.lastWasDup = false
	if slot.seenPacket && !jumpAllowed && p.Header.ContinuityCounter != (slot.lastCC+1)%16 {
		reason := ts.ErrContinuityGap
		if discontinuity {
			reason = ts.ErrDiscontinuity
		}
		if slot.started {
			out = a.tear(slot, p.Header.PID, p.Offset, reason, 0, out)
		} else if a.report != nil {
			// A PSI PID closes its unit in every packet, so a gap between two whole units loses no bytes and still breaks the counter.
			a.report(ts.RecoverableError{
				Kind: ts.ErrorKindContinuity, PID: p.Header.PID, Offset: p.Offset, Err: reason,
			})
		}
	}

	if p.Header.PayloadUnitStartIndicator {
		if slot.started {
			if slot.isPSI {
				a.carrySectionTail(slot, p)
			}
			if u, ok := a.finish(slot, p.Header.PID, p.Offset); ok {
				out = append(out, u)
			}
		}
		a.startUnit(slot, p)
	} else if !slot.started {
		// Joined past its start indicator: accumulated only to be counted.
		a.startUnit(slot, p)
		slot.headless = true
	}
	// After finish/start: a tear clears seenPacket, and the unit this packet opens must still see its own repeat
	slot.lastCC = p.Header.ContinuityCounter
	slot.seenPacket = true

	if need := len(slot.buf.bs) + len(p.Payload); need > cap(slot.buf.bs) {
		if overLimit(need, a.limitOf(slot.isPSI)) {
			return a.tear(slot, p.Header.PID, p.Offset, ts.ErrUnitTooLarge, len(p.Payload), out)
		}
		slot.grow(need)
	}
	slot.buf.bs = append(slot.buf.bs, p.Payload...)
	slot.lastLen = len(p.Payload)
	slot.lastOffset = p.Offset
	slot.lastTag = p.Tag

	if slot.isPSI && !slot.headless && slot.psiComplete() {
		if u, ok := a.finish(slot, p.Header.PID, p.Offset); ok {
			out = append(out, u)
		}
	}
	return out
}

// §2.4.4.2: the bytes ahead of pointer_field finish the section the previous packet left open, and stay at the head of the new unit too, where pointer_field skips them.
func (a *accumulator) carrySectionTail(slot *pidSlot, p *ts.Packet) {
	payload := p.Payload
	if len(payload) == 0 {
		return
	}
	end := 1 + int(payload[0])
	if end <= 1 || end > len(payload) {
		return
	}
	if need := len(slot.buf.bs) + end - 1; need > cap(slot.buf.bs) {
		if overLimit(need, a.limitOf(slot.isPSI)) {
			return
		}
		slot.grow(need)
	}
	slot.buf.bs = append(slot.buf.bs, payload[1:end]...)
	slot.lastOffset, slot.lastTag = p.Offset, p.Tag
}

// never call after appending the repeat: lastLen must still describe the buffer tail
func (s *pidSlot) differsFromTail(payload []byte) bool {
	return s.started && s.lastLen > 0 && s.lastLen <= len(s.buf.bs) &&
		!bytes.Equal(payload, s.buf.bs[len(s.buf.bs)-s.lastLen:])
}

func (a *accumulator) checkDuplicate(slot *pidSlot, p *ts.Packet) {
	if a.report != nil && slot.differsFromTail(p.Payload) {
		a.report(ts.RecoverableError{
			Kind: ts.ErrorKindPacketDrop, PID: p.Header.PID, Offset: p.Offset,
			Dropped: int64(len(p.Payload)), Err: ts.ErrDuplicateMismatch,
		})
	}
}

func (a *accumulator) limitOf(isPSI bool) int {
	if isPSI {
		return a.maxPSI
	}
	return a.maxPES
}

func overLimit(size, limit int) bool {
	return limit >= 0 && size > limit
}

func (a *accumulator) finish(slot *pidSlot, pid uint16, offset int64) (u unit, ok bool) {
	if !slot.started {
		return
	}
	if overLimit(len(slot.buf.bs), a.limitOf(slot.isPSI)) {
		a.tear(slot, pid, offset, ts.ErrUnitTooLarge, 0, nil)
		return
	}
	return slot.flush(pid)
}

func (a *accumulator) tear(slot *pidSlot, pid uint16, offset int64, reason error, lost int, out []unit) []unit {
	if !slot.started {
		return out
	}
	// A whole PES lost only the payload after it; ErrUnitTooLarge stays torn, as finish would tear it back and recurse.
	if !errors.Is(reason, ts.ErrUnitTooLarge) && slot.pesWhole() {
		if u, ok := a.finish(slot, pid, offset); ok {
			out = append(out, u)
			if a.report != nil {
				e := ts.RecoverableError{Kind: ts.ErrorKindContinuity, PID: pid, Offset: offset, Err: reason}
				if lost > 0 {
					e.Kind, e.Dropped = ts.ErrorKindPacketDrop, int64(lost)
				}
				a.report(e)
			}
		}
		slot.seenPacket = false
		return out
	}
	if a.report != nil && len(slot.buf.bs)+lost > 0 {
		a.report(ts.RecoverableError{
			Kind: ts.ErrorKindTornUnit, PID: pid, Offset: offset,
			Dropped: int64(len(slot.buf.bs) + lost), Err: reason,
		})
	}
	slot.release()
	slot.seenPacket = false
	return out
}

func (s *pidSlot) start(p *ts.Packet, isPSI bool, limit int) {
	s.started = true
	s.headless = false
	s.isPSI = isPSI
	s.firstCC = p.Header.ContinuityCounter
	s.firstOffset = p.Offset
	s.firstTag = p.Tag
	s.psiScan = 0
	if p.Header.HasAdaptationField {
		s.afIdx ^= 1
		s.af[s.afIdx].CopyFrom(&p.AdaptationField)
		s.hasAF = true
	} else {
		s.hasAF = false
	}

	if s.buf == nil {
		class := s.classFor(p.Payload, isPSI)
		if limit > 0 {
			class = min(class, classOf(limit))
		}
		s.buf = poolOfPayload.getClass(class)
	}
	s.buf.bs = s.buf.bs[:0]
}

func (s *pidSlot) classFor(payload []byte, isPSI bool) uint8 {
	if isPSI {
		return maxClass(s.sticky, defaultFloorClass)
	}
	if len(payload) >= pes.HeaderSize && isPESPayload(payload) {
		if pl := binary.BigEndian.Uint16(payload[pesLengthOffset : pesLengthOffset+2]); pl > 0 {
			return maxClass(classOf(int(pl)+pes.HeaderSize), s.sticky)
		}
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

func (s *pidSlot) flush(pid uint16) (u unit, ok bool) {
	if !s.started || len(s.buf.bs) == 0 {
		s.release()
		return
	}
	s.sticky = maxClass(s.sticky, classOf(len(s.buf.bs)))
	if !s.isPSI && !s.headless {
		s.sawPES = true
	}
	u = unit{buf: s.buf, cc: s.firstCC, pid: pid, isPSI: s.isPSI, headless: s.headless, PacketSpan: PacketSpan{
		FirstPacketOffset: s.firstOffset, LastPacketOffset: s.lastOffset,
		FirstPacketTag: s.firstTag, LastPacketTag: s.lastTag,
	}}
	if s.hasAF {
		u.af = &s.af[s.afIdx]
	}
	s.buf = nil
	s.started = false
	s.lastLen = 0
	return u, true
}

func (s *pidSlot) pesWhole() bool {
	b := s.buf.bs
	if s.isPSI || s.headless || len(b) < pes.HeaderSize || !isPESPayload(b) {
		return false
	}
	n := int(binary.BigEndian.Uint16(b[pesLengthOffset : pesLengthOffset+2]))
	return n != 0 && len(b) >= pes.HeaderSize+n
}

func (s *pidSlot) release() {
	if s.buf != nil {
		poolOfPayload.put(s.buf)
		s.buf = nil
	}
	s.started = false
	s.lastLen = 0
}

const psiSectionHeaderLen = 3

// psiScan carries across calls; start() resets it per unit.
func (s *pidSlot) psiComplete() bool {
	bs := s.buf.bs
	if s.psiScan == 0 {
		if len(bs) == 0 {
			return false
		}
		s.psiScan = 1 + int(bs[0]) // pointer_field
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

func (a *accumulator) close() {
	for i := range a.slots.Vals {
		a.slots.Vals[i].release()
	}
}

func classOf(size int) uint8 {
	if size <= 1<<classShift {
		return 0
	}
	return uint8(bits.Len(uint(size-1)) - classShift)
}

func maxClass(a, b uint8) uint8 {
	if a > b {
		return a
	}
	return b
}
