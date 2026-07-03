package astits

// pidMap — компактный ассоциативный контейнер под единицы ключей (PID'ы):
// ключи в отдельном слайсе — скан умещается в одну кэшлинию, значение трогается
// только по найденному индексу. Дешевле map-хэша на каждый пакет и зануления
// плоского массива на каждый короткоживущий инстанс.
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

// get возвращает указатель на значение или nil, если ключа нет. Указатель
// валиден до следующего getOrAdd/remove (append реаллоцирует).
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
