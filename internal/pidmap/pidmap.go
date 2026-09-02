// Package pidmap is a compact associative container for PIDs: keys and values
// live in parallel slices and a key is found through a small open-addressed
// index of positions, so the common lookup is one hashed probe whatever the
// number of PIDs. The index starts in an inline array, so a short-lived Map
// costs no allocation, and moves to the heap once the load would lengthen
// the probe chains.
package pidmap

const (
	inlineIndex = 64
	hashMul     = 2654435761 // Knuth's multiplicative hashing constant
	posNone     = 0          // index entries hold position+1, so a cleared entry is empty
)

type Map[V any] struct {
	Keys []uint16
	Vals []V

	idx    []uint16 // nil until the first key; len is a power of two
	shift  uint8    // 32 - log2(len(idx))
	idxArr [inlineIndex]uint16
}

func New[V any](capacity int) Map[V] {
	return Map[V]{
		Keys: make([]uint16, 0, capacity),
		Vals: make([]V, 0, capacity),
	}
}

func hash(key uint16, shift uint8) uint32 {
	return (uint32(key) * hashMul) >> shift
}

// Get returns a pointer to the value, or nil if the key is absent. The pointer
// is valid until the next GetOrAdd/Remove (append reallocates). This is the
// per-packet path and must stay inlinable: everything rarer lives elsewhere.
func (m *Map[V]) Get(key uint16) *V {
	if m.idx == nil {
		return nil
	}
	mask := uint32(len(m.idx) - 1)
	for s := hash(key, m.shift); ; s = (s + 1) & mask {
		p := m.idx[s]
		if p == posNone {
			return nil
		}
		if m.Keys[p-1] == key {
			return &m.Vals[p-1]
		}
	}
}

func (m *Map[V]) GetOrAdd(key uint16) *V {
	if v := m.Get(key); v != nil {
		return v
	}
	return m.add(key)
}

func (m *Map[V]) add(key uint16) *V {
	var zero V
	m.Keys = append(m.Keys, key)
	m.Vals = append(m.Vals, zero)
	if n := len(m.Keys); m.idx == nil || n > len(m.idx)/2 {
		m.rebuild()
	} else {
		m.index(n - 1)
	}
	return &m.Vals[len(m.Vals)-1]
}

func (m *Map[V]) index(pos int) {
	mask := uint32(len(m.idx) - 1)
	for s := hash(m.Keys[pos], m.shift); ; s = (s + 1) & mask {
		if m.idx[s] == posNone {
			m.idx[s] = uint16(pos + 1)
			return
		}
	}
}

// rebuild sizes the index for the keys at load ≤ 1/4 (the inline array while
// it fits) and indexes every key again.
func (m *Map[V]) rebuild() {
	size := inlineIndex
	for size < 4*len(m.Keys) {
		size *= 2
	}
	switch {
	case size == inlineIndex:
		m.idxArr = [inlineIndex]uint16{}
		m.idx = m.idxArr[:]
	case size == len(m.idx):
		clear(m.idx)
	default:
		m.idx = make([]uint16, size)
	}
	m.shift = 32
	for bits := size; bits > 1; bits >>= 1 {
		m.shift--
	}
	for i := range m.Keys {
		m.index(i)
	}
}

func (m *Map[V]) Remove(key uint16) {
	v := m.Get(key)
	if v == nil {
		return
	}
	i := 0
	for j := range m.Vals {
		if &m.Vals[j] == v {
			i = j
			break
		}
	}
	m.Keys = append(m.Keys[:i], m.Keys[i+1:]...)
	m.Vals = append(m.Vals[:i], m.Vals[i+1:]...)
	m.rebuild()
}

// Clear drops every entry, keeping the backing storage; the values are zeroed
// so nothing they referenced stays alive.
func (m *Map[V]) Clear() {
	clear(m.Vals)
	m.Keys = m.Keys[:0]
	m.Vals = m.Vals[:0]
	clear(m.idx)
}

func (m *Map[V]) Has(key uint16) bool {
	return m.Get(key) != nil
}

func (m *Map[V]) Set(key uint16, val V) {
	*m.GetOrAdd(key) = val
}
