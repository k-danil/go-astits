package ts

// 1 KiB: pointer receivers only, a value receiver copies the whole set per call.
type PIDSet [8192 / 64]uint64

// Guards an out-of-range pid from indexing past the set.
const pidMask = 0x1fff

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
