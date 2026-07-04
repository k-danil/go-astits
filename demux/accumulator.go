package demux

import (
	"encoding/binary"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
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

	lastCC         uint8
	lastHadPayload bool
	seenPacket     bool

	sticky  uint8 // sticky-max size class over the slot's lifetime
	started bool
	isPSI   bool
	stats   uint32
}

// accumulator replaces the per-PID packet lists: it owns per-PID slots and
// flushes completed units as contiguous buffers.
type accumulator struct {
	slots      pidmap.Map[pidSlot]
	programMap *pidmap.Map[uint16]
	dvbTables  bool

	keysArr [packetPoolPreallocPIDs]uint16
	valsArr [packetPoolPreallocPIDs]pidSlot
}

const packetPoolPreallocPIDs = 8

func (a *accumulator) init(programMap *pidmap.Map[uint16], dvbTables bool) {
	a.slots = pidmap.Map[pidSlot]{Keys: a.keysArr[:0], Vals: a.valsArr[:0]}
	a.programMap = programMap
	a.dvbTables = dvbTables
}

// unit is a flushed payload unit handed to the parse stage. buf ownership
// moves to the receiver.
type unit struct {
	buf   *dataPayload
	af    *ts.PacketAdaptationField
	cc    uint8
	pid   uint16
	isPSI bool
}

func (a *accumulator) isPSIPID(pid uint16) bool {
	return pid == ts.PIDPAT ||
		a.programMap.Has(pid) ||
		(a.dvbTables && ((pid >= 0x10 && pid <= 0x14) || (pid >= 0x1e && pid <= 0x1f)))
}

// add consumes the packet's payload and appends completed units (zero, one,
// or — for a torn PSI flushed by the same packet that completes the next
// section — two) to out. Buffer ownership moves with the units.
func (a *accumulator) add(p *ts.Packet, out []unit) []unit {
	if p.Header.TransportErrorIndicator || !p.Header.HasPayload {
		return out
	}

	slot := a.slots.GetOrAdd(p.Header.PID)
	slot.stats++

	// Same packet repeated (retransmission)
	if slot.seenPacket && p.Header.ContinuityCounter == slot.lastCC && slot.lastHadPayload {
		return out
	}
	// Discontinuity drops the unfinished unit
	if slot.started && a.discontinuity(slot, p) {
		slot.release()
	}
	slot.lastCC = p.Header.ContinuityCounter
	slot.lastHadPayload = p.Header.HasPayload
	slot.seenPacket = true

	if p.Header.PayloadUnitStartIndicator {
		if slot.started {
			if u, ok := slot.flush(p.Header.PID); ok {
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

	slot.append(p.Payload)

	// A PSI unit completes by section lengths, without waiting for the next
	// PayloadUnitStartIndicator
	if slot.isPSI && slot.psiComplete() {
		if u, ok := slot.flush(p.Header.PID); ok {
			out = append(out, u)
		}
	}
	return out
}

func (a *accumulator) discontinuity(slot *pidSlot, p *ts.Packet) bool {
	if p.Header.HasAdaptationField && p.AdaptationField.DiscontinuityIndicator {
		return true
	}
	if p.Header.HasPayload {
		return p.Header.ContinuityCounter != (slot.lastCC+1)%16
	}
	return p.Header.ContinuityCounter != slot.lastCC
}

// start begins a new unit from a PayloadUnitStartIndicator packet.
func (s *pidSlot) start(p *ts.Packet, isPSI bool) {
	s.started = true
	s.isPSI = isPSI
	s.cc = p.Header.ContinuityCounter
	if p.Header.HasAdaptationField {
		s.afIdx ^= 1
		s.af[s.afIdx].CopyFrom(p.AdaptationField)
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

// append copies the payload into the unit buffer, growing by size classes.
func (s *pidSlot) append(payload []byte) {
	need := len(s.buf.bs) + len(payload)
	if need > cap(s.buf.bs) {
		grown := poolOfPayload.getClass(classOf(need))
		grown.bs = grown.bs[:len(s.buf.bs)]
		copy(grown.bs, s.buf.bs)
		poolOfPayload.put(s.buf)
		s.buf = grown
	}
	s.buf.bs = append(s.buf.bs, payload...)
}

// flush hands the accumulated unit over; the slot remembers the size class
// for the next unit of this PID.
func (s *pidSlot) flush(pid uint16) (u unit, ok bool) {
	if !s.started || len(s.buf.bs) == 0 {
		s.release()
		return
	}
	s.sticky = maxClass(s.sticky, classOf(len(s.buf.bs)))
	u = unit{buf: s.buf, cc: s.cc, pid: pid, isPSI: s.isPSI}
	if s.hasAF {
		u.af = &s.af[s.afIdx]
	}
	s.buf = nil
	s.started = false
	return u, true
}

func (s *pidSlot) release() {
	if s.buf != nil {
		poolOfPayload.put(s.buf)
		s.buf = nil
	}
	s.started = false
}

// psiComplete reports whether the accumulated buffer already holds all its
// sections: a scan over lengths, no copies.
func (s *pidSlot) psiComplete() bool {
	i := bytesiter.New(s.buf.bs)

	b, err := i.NextByte()
	if err != nil {
		return false
	}
	i.Skip(int(b)) // pointer filler bytes

	for i.HasBytesLeft() {
		if b, err = i.NextByte(); err != nil {
			return false
		}
		if psi.TableID(b).StopsParsing() {
			break
		}
		var bs []byte
		if bs, err = i.NextBytesNoCopy(2); err != nil {
			return false
		}
		i.Skip(int(binary.BigEndian.Uint16(bs) & 0x0fff))
	}

	return i.Len() >= i.Offset()
}

// drain flushes the unfinished unit of the lowest PID that has one: EOF tails
// come out in ascending PID order.
func (a *accumulator) drain() (u unit, ok bool) {
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
	return a.slots.Vals[minIdx].flush(a.slots.Keys[minIdx])
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
