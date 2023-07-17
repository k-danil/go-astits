package astits

// programMap represents a program ids map
type programMap struct {
	p map[uint64]uint16 // map[ProgramMapID]ProgramNumber
}

// newProgramMap creates a new program ids map
func newProgramMap() *programMap {
	return &programMap{
		p: make(map[uint64]uint16),
	}
}

// existsUnlocked checks whether the program with this pid exists
func (m programMap) existsUnlocked(pid uint16) (ok bool) {
	if len(m.p) > 0 {
		_, ok = m.p[uint64(pid)]
	}
	return
}

// setUnlocked sets a new program id
func (m programMap) setUnlocked(pid, number uint16) {
	m.p[uint64(pid)] = number
}

func (m programMap) unsetUnlocked(pid uint16) {
	delete(m.p, uint64(pid))
}

func (m programMap) toPATDataUnlocked() *PATData {
	d := &PATData{
		Programs:          make([]PATProgram, 0, len(m.p)),
		TransportStreamID: uint16(PSITableIDPAT),
	}

	for pid, pnr := range m.p {
		d.Programs = append(d.Programs, PATProgram{
			ProgramMapID:  uint16(pid),
			ProgramNumber: pnr,
		})
	}

	return d
}
