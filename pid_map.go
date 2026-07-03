package astits

// pidMap is a compact associative container for a handful of keys (PIDs):
// keys live in a separate slice so a scan fits in a single cache line, and the
// value is touched only via the found index. Cheaper than a map hash per packet
// and than zeroing a flat array per short-lived instance.
type pidMap[V any] struct {
	keys []uint16
	vals []V
}

func newPidMap[V any](capacity int) pidMap[V] {
	return pidMap[V]{
		keys: make([]uint16, 0, capacity),
		vals: make([]V, 0, capacity),
	}
}

// get returns a pointer to the value, or nil if the key is absent. The pointer
// is valid until the next getOrAdd/remove (append reallocates).
func (m *pidMap[V]) get(key uint16) *V {
	for i, k := range m.keys {
		if k == key {
			return &m.vals[i]
		}
	}
	return nil
}

func (m *pidMap[V]) getOrAdd(key uint16) *V {
	if v := m.get(key); v != nil {
		return v
	}
	var zero V
	m.keys = append(m.keys, key)
	m.vals = append(m.vals, zero)
	return &m.vals[len(m.vals)-1]
}

func (m *pidMap[V]) remove(key uint16) {
	for i, k := range m.keys {
		if k == key {
			m.keys = append(m.keys[:i], m.keys[i+1:]...)
			m.vals = append(m.vals[:i], m.vals[i+1:]...)
			return
		}
	}
}
