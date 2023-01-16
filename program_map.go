package astits

// programMap represents a program ids map
type programMap struct {
	p map[uint16]uint16 // map[ProgramMapID]ProgramNumber
}

// newProgramMap creates a new program ids map
func newProgramMap() *programMap {
	return &programMap{
		p: make(map[uint16]uint16),
	}
}

// exists checks whether the program with this pid exists
func (m programMap) exists(pid uint16) (ok bool) {
	_, ok = m.p[pid]
	return
}

// set sets a new program id
func (m programMap) set(pid, number uint16) {
	m.p[pid] = number
}

func (m programMap) unset(pid uint16) {
	delete(m.p, pid)
}

func (m programMap) toPATData() *PATData {
	d := &PATData{
		Programs:          make([]PATProgram, 0, len(m.p)),
		TransportStreamID: uint16(PSITableIDPAT),
	}

	for pid, pnr := range m.p {
		d.Programs = append(d.Programs, PATProgram{
			ProgramMapID:  pid,
			ProgramNumber: pnr,
		})
	}

	return d
}
