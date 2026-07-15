package ts

// PIDSet is a bit set over the 13-bit PID space (0..8191). Used as an inline
// allow-list the demuxer checks in its packet parse hot path — a couple of loads
// and a bit test, without the call and capture overhead of a PacketSkipper.
//
// All methods take a pointer receiver: the set is 1 KiB, so a value receiver
// would copy it on every call.
type PIDSet [8192 / 64]uint64

// pidMask folds a value into the 13-bit PID space so an out-of-range argument
// cannot index past the set (matching the on-wire PID the parser extracts).
const pidMask = 0x1fff

// NewPIDSet returns a set containing pids.
func NewPIDSet(pids ...uint16) (s PIDSet) {
	for _, pid := range pids {
		s.Add(pid)
	}
	return
}

func (s *PIDSet) Add(pid uint16) {
	pid &= pidMask
	s[pid>>6] |= uint64(1) << (pid & 63)
}

func (s *PIDSet) Remove(pid uint16) {
	pid &= pidMask
	s[pid>>6] &^= uint64(1) << (pid & 63)
}

func (s *PIDSet) Has(pid uint16) bool {
	pid &= pidMask
	return s[pid>>6]&(uint64(1)<<(pid&63)) != 0
}

func (s *PIDSet) Clear() {
	*s = PIDSet{}
}
