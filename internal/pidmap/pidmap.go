// Package pidmap is a compact associative container for a handful of keys (PIDs):
// keys live in a separate slice so a scan fits in a single cache line, and the
// value is touched only via the found index. Cheaper than a map hash per packet
// and than zeroing a flat array per short-lived instance.
package pidmap

type Map[V any] struct {
	Keys []uint16
	Vals []V
}

func New[V any](capacity int) Map[V] {
	return Map[V]{
		Keys: make([]uint16, 0, capacity),
		Vals: make([]V, 0, capacity),
	}
}

// Get returns a pointer to the value, or nil if the key is absent. The pointer
// is valid until the next GetOrAdd/Remove (append reallocates).
func (m *Map[V]) Get(key uint16) *V {
	for i, k := range m.Keys {
		if k == key {
			return &m.Vals[i]
		}
	}
	return nil
}

func (m *Map[V]) GetOrAdd(key uint16) *V {
	if v := m.Get(key); v != nil {
		return v
	}
	var zero V
	m.Keys = append(m.Keys, key)
	m.Vals = append(m.Vals, zero)
	return &m.Vals[len(m.Vals)-1]
}

func (m *Map[V]) Remove(key uint16) {
	for i, k := range m.Keys {
		if k == key {
			m.Keys = append(m.Keys[:i], m.Keys[i+1:]...)
			m.Vals = append(m.Vals[:i], m.Vals[i+1:]...)
			return
		}
	}
}

func (m *Map[V]) Has(key uint16) bool {
	return m.Get(key) != nil
}

func (m *Map[V]) Set(key uint16, val V) {
	*m.GetOrAdd(key) = val
}
