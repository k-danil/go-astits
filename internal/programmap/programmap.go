package programmap

// Map represents a program ids map (pid -> program number)
type Map struct {
	P map[uint64]uint16
}

func New() *Map {
	return &Map{P: make(map[uint64]uint16)}
}

// ExistsUnlocked checks whether the program with this pid exists
func (m Map) ExistsUnlocked(pid uint16) (ok bool) {
	if len(m.P) > 0 {
		_, ok = m.P[uint64(pid)]
	}
	return
}

// SetUnlocked sets a new program id
func (m Map) SetUnlocked(pid, number uint16) {
	m.P[uint64(pid)] = number
}

func (m Map) UnsetUnlocked(pid uint16) {
	delete(m.P, uint64(pid))
}
