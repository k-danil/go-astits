package mux

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/k-danil/go-astits/v2/internal/pidmap"
	"github.com/k-danil/go-astits/v2/pes"
	"github.com/k-danil/go-astits/v2/psi"
	"github.com/k-danil/go-astits/v2/ts"
)

const (
	startPID           uint16 = 0x0100
	pmtStartPID        uint16 = 0x1000
	programNumberStart uint16 = 1
	packetMaxPayload          = 184
)

var (
	ErrPIDNotFound      = errors.New("astits: PID not found")
	ErrPIDAlreadyExists = errors.New("astits: PID already exists")
	ErrPCRPIDInvalid    = errors.New("astits: PCR PID invalid")
)

// Muxer writes an MPEG-TS stream for a single program.
type Muxer struct {
	ctx context.Context
	w   io.Writer

	packetSize             int
	tablesRetransmitPeriod int // period in PES packets

	pm         pidmap.Map[uint16] // pid -> programNumber
	pmt        psi.PMT
	patVersion wrappingCounter
	pmtVersion wrappingCounter
	patCC      wrappingCounter
	pmtCC      wrappingCounter
	nextPID    uint16
	pmUpdated  bool
	pmtUpdated bool

	patBytes bytes.Buffer
	pmtBytes bytes.Buffer

	pkt     []byte
	stuffAF ts.PacketAdaptationField
	pktArr  [ts.PacketSize]byte

	patData []byte
	pmtData []byte

	esContexts              pidmap.Map[esContext]
	tablesRetransmitCounter int

	// Inline storage, each paired with a field above to keep a fresh muxer's
	// tables and small maps off the heap.
	pmKeysArr [4]uint16    // pm keys
	pmValsArr [4]uint16    // pm vals
	esKeysArr [8]uint16    // esContexts keys
	esValsArr [8]esContext // esContexts vals
	patArr    [ts.PacketSize]byte
	pmtArr    [ts.PacketSize]byte
}

type esContext struct {
	es *psi.ElementaryStream
	cc wrappingCounter
}

// WithTablesRetransmitPeriod sets how often PAT/PMT are re-emitted, counted in
// written PES packets.
func WithTablesRetransmitPeriod(newPeriod int) func(*Muxer) {
	return func(m *Muxer) {
		m.tablesRetransmitPeriod = newPeriod
	}
}

// TODO MuxerOptAutodetectPCRPID selecting first video PID for each PMT, falling back to first audio, falling back to any other

// New creates a muxer writing to w; register streams with AddElementaryStream
// before writing data.
func New(ctx context.Context, w io.Writer, opts ...func(*Muxer)) (m *Muxer) {
	m = &Muxer{
		ctx: ctx,
		w:   w,

		packetSize:             ts.PacketSize, // no 192-byte packet support yet
		tablesRetransmitPeriod: 40,

		pmt: psi.PMT{
			ElementaryStreams: []psi.ElementaryStream{},
			ProgramNumber:     programNumberStart,
		},

		// table version is 5-bit field
		patVersion: newWrappingCounter(0b11111),
		pmtVersion: newWrappingCounter(0b11111),

		patCC: newWrappingCounter(0b1111),
		pmtCC: newWrappingCounter(0b1111),
	}

	m.pkt = m.pktArr[:]
	m.pm = pidmap.Map[uint16]{Keys: m.pmKeysArr[:0], Vals: m.pmValsArr[:0]}
	m.esContexts = pidmap.Map[esContext]{Keys: m.esKeysArr[:0], Vals: m.esValsArr[:0]}
	m.patData = m.patArr[:0]
	m.pmtData = m.pmtArr[:0]

	// TODO multiple programs support
	m.pm.Set(pmtStartPID, programNumberStart)
	m.pmUpdated = true

	for _, opt := range opts {
		opt(m)
	}

	// to output tables at the very start
	m.tablesRetransmitCounter = m.tablesRetransmitPeriod

	return
}

// if es.ElementaryPID is zero, it will be generated automatically
func (m *Muxer) AddElementaryStream(es psi.ElementaryStream) error {
	if es.ElementaryPID != 0 {
		for _, oes := range m.pmt.ElementaryStreams {
			if oes.ElementaryPID == es.ElementaryPID {
				return ErrPIDAlreadyExists
			}
		}
	} else {
		es.ElementaryPID = m.nextPID
		m.nextPID++
	}

	m.pmt.ElementaryStreams = append(m.pmt.ElementaryStreams, es)

	*m.esContexts.GetOrAdd(es.ElementaryPID) = esContext{
		es: &es,
		cc: newWrappingCounter(0b1111), // CC is 4 bits
	}
	// invalidate pmt cache
	m.pmtBytes.Reset()
	m.pmtUpdated = true
	return nil
}

func (m *Muxer) RemoveElementaryStream(pid uint16) error {
	foundIdx := -1
	for i, oes := range m.pmt.ElementaryStreams {
		if oes.ElementaryPID == pid {
			foundIdx = i
			break
		}
	}

	if foundIdx == -1 {
		return ErrPIDNotFound
	}

	m.pmt.ElementaryStreams = append(m.pmt.ElementaryStreams[:foundIdx], m.pmt.ElementaryStreams[foundIdx+1:]...)
	m.esContexts.Remove(pid)
	m.pmtBytes.Reset()
	m.pmtUpdated = true
	return nil
}

// SetPCRPID marks pid as one to look PCRs in
func (m *Muxer) SetPCRPID(pid uint16) {
	m.pmt.PCRPID = pid
	m.pmtUpdated = true
}

// SetCC seeds the continuity counter for a PID so passthrough output continues
// the source packet sequence without a discontinuity.
func (m *Muxer) SetCC(pid uint16, cc uint8) error {
	ctx := m.esContexts.Get(pid)
	if ctx == nil {
		return ErrPIDNotFound
	}
	return ctx.cc.set(int(cc))
}

// WriteData writes Data to TS stream
// Currently only PES packets are supported
// Be aware that after successful call WriteData will set d.AdaptationField.StuffingLength value to zero
func (m *Muxer) WriteData(d *Data) (bytesWritten int, err error) {
	ctx := m.esContexts.Get(d.PID)
	if ctx == nil {
		return 0, ErrPIDNotFound
	}

	forceTables := d.AdaptationField != nil &&
		d.AdaptationField.RandomAccessIndicator &&
		d.PID == m.pmt.PCRPID

	var n int
	if n, err = m.retransmitTables(forceTables); err != nil {
		return n, err
	}

	bytesWritten += n

	payloadStart := true
	writeAf := d.AdaptationField != nil
	payloadBytesWritten := 0
	for payloadBytesWritten < len(d.PES.Data) {
		pktLen := ts.HeaderSize
		pkt := ts.Packet{
			Header: ts.PacketHeader{
				ContinuityCounter:         uint8(ctx.cc.inc()),
				HasAdaptationField:        writeAf,
				HasPayload:                false,
				PayloadUnitStartIndicator: false,
				PID:                       d.PID,
			},
		}

		if writeAf {
			pkt.AdaptationField = d.AdaptationField
			// one byte for adaptation field length field
			pktLen += 1 + d.AdaptationField.CalcLength()
			writeAf = false
		}

		bytesAvailable := m.packetSize - pktLen
		if payloadStart {
			pesHeaderLengthCurrent := pes.HeaderSize + d.PES.Header.OptionalHeader.CalcLength()
			// d.AdaptationField with pes header are too big, we don't have space to write pes header
			if bytesAvailable < pesHeaderLengthCurrent {
				pkt.Header.HasAdaptationField = true
				if pkt.AdaptationField == nil {
					pkt.AdaptationField = m.stuffingAdaptationField(bytesAvailable)
				} else {
					pkt.AdaptationField.StuffingLength = uint8(bytesAvailable)
				}
			} else {
				pkt.Header.HasPayload = true
				pkt.Header.PayloadUnitStartIndicator = true
			}
		} else {
			pkt.Header.HasPayload = true
		}

		if pkt.Header.HasPayload {
			if d.PES.Header.StreamID == 0 {
				d.PES.Header.StreamID = ctx.es.StreamType.ToPESStreamID()
			}

			// Size the chunk analytically so the AF stuffing is finalized
			// before serialization; the PES then lands in the packet tail
			// (payloadOffset = packetSize - ntot) with no scratch copy.
			ntot, npayload := d.PES.Header.CalcDataLength(
				d.PES.Data[payloadBytesWritten:],
				payloadStart,
				bytesAvailable,
			)

			bytesAvailable -= ntot
			// if we still have some space in packet, we should stuff it with adaptation field stuffing
			// we can't stuff packets with 0xff at the end of a packet since it's not uncommon for PES payloads to have length unspecified
			if bytesAvailable > 0 {
				pkt.Header.HasAdaptationField = true
				if pkt.AdaptationField == nil {
					pkt.AdaptationField = m.stuffingAdaptationField(bytesAvailable)
				} else {
					pkt.AdaptationField.StuffingLength = uint8(bytesAvailable)
				}
			}

			payloadOffset := m.packetSize - ntot
			if _, _, err = d.PES.Header.Put(
				m.pkt[payloadOffset:m.packetSize],
				d.PES.Data[payloadBytesWritten:],
				payloadStart,
			); err != nil {
				return
			}
			payloadBytesWritten += npayload

			// Assemble header + AF ahead of the payload directly in m.pkt;
			// bypassing pkt.Put keeps the already-placed payload from being
			// copied a second time.
			pkt.Header.Put(m.pkt)
			if pkt.Header.HasAdaptationField {
				if _, err = pkt.AdaptationField.Put(m.pkt[ts.HeaderSize:]); err != nil {
					return
				}
			}

			if n, err = m.w.Write(m.pkt); err != nil {
				return
			}

			bytesWritten += n

			payloadStart = false
		}
	}

	if d.AdaptationField != nil {
		d.AdaptationField.StuffingLength = 0
	}

	return
}

// Writes given packet to MPEG-TS stream
// Stuffs with 0xffs if packet turns out to be shorter than target packet length
func (m *Muxer) WritePacket(p *ts.Packet) (int, error) {
	if raw := p.Raw(); len(raw) > 0 {
		return m.w.Write(raw)
	}
	if _, err := p.Put(m.pkt); err != nil {
		return 0, err
	}
	return m.w.Write(m.pkt)
}

// stuffingAdaptationField reuses the muxer's scratch AF: no allocation per stuffed
// packet, Reset() guarantees cleanliness between uses
func (m *Muxer) stuffingAdaptationField(bytesToStuff int) *ts.PacketAdaptationField {
	m.stuffAF.Reset()
	if bytesToStuff == 1 {
		m.stuffAF.IsOneByteStuffing = true
	} else {
		// one byte for length and one for flags
		m.stuffAF.StuffingLength = uint8(bytesToStuff - 2)
	}
	return &m.stuffAF
}

func (m *Muxer) retransmitTables(force bool) (n int, err error) {
	m.tablesRetransmitCounter++
	if !force && m.tablesRetransmitCounter < m.tablesRetransmitPeriod {
		return
	}

	if n, err = m.WriteTables(); err != nil {
		return
	}

	m.tablesRetransmitCounter = 0
	return
}

// WriteTables writes the PAT and the PMT for the registered program.
func (m *Muxer) WriteTables() (bytesWritten int, err error) {
	if err = m.generatePAT(); err != nil {
		return
	}

	if err = m.generatePMT(); err != nil {
		return
	}

	var n int
	if n, err = m.w.Write(m.patBytes.Bytes()); err != nil {
		return
	}
	bytesWritten += n

	if n, err = m.w.Write(m.pmtBytes.Bytes()); err != nil {
		return
	}
	bytesWritten += n

	return
}

// maxPATProgramsPerSection is how many 4-byte program entries fit a section
// next to the syntax header and CRC32.
const maxPATProgramsPerSection = (1021 - 5 - 4) / 4

func (m *Muxer) generatePAT() (err error) {
	if m.pmUpdated {
		d := toPATData(&m.pm)

		numSections := (len(d.Programs) + maxPATProgramsPerSection - 1) / maxPATProgramsPerSection
		if numSections == 0 {
			numSections = 1
		}
		version := uint8(m.patVersion.inc())

		psiData := psi.Data{Sections: make([]psi.Section, 0, numSections)}
		for si := 0; si < numSections; si++ {
			part := &psi.PAT{TransportStreamID: d.TransportStreamID}
			end := min((si+1)*maxPATProgramsPerSection, len(d.Programs))
			part.Programs = d.Programs[si*maxPATProgramsPerSection : end]

			psiData.Sections = append(psiData.Sections, psi.Section{
				Header: psi.SectionHeader{
					SectionLength:          uint16(part.CalcSectionLength()),
					SectionSyntaxIndicator: true,
					TableID:                psi.TableID(d.TransportStreamID),
				},
				Syntax: &psi.SectionSyntax{
					Data: part,
					Header: psi.SectionSyntaxHeader{
						CurrentNextIndicator: true,
						SectionNumber:        uint8(si),
						LastSectionNumber:    uint8(numSections - 1),
						TableIDExtension:     d.TransportStreamID,
						VersionNumber:        version,
					},
				},
			})
		}

		if m.patData, err = psiData.Append(m.patData[:0]); err != nil {
			return
		}

		m.pmUpdated = false
	}

	m.patBytes.Reset()

	l := len(m.patData)
	for i := 0; i <= l/packetMaxPayload; i++ {
		start := i * packetMaxPayload
		stop := min(start+packetMaxPayload, l)
		pkt := ts.Packet{
			Header: ts.PacketHeader{
				HasPayload:                true,
				PayloadUnitStartIndicator: i == 0,
				PID:                       ts.PIDPAT,
				ContinuityCounter:         uint8(m.patCC.inc()),
			},
			Payload: m.patData[start:stop],
		}
		if _, err = pkt.Put(m.pkt); err != nil {
			return
		}
		m.patBytes.Write(m.pkt)
	}

	return
}

func (m *Muxer) generatePMT() (err error) {
	if m.pmtUpdated {
		hasPCRPID := false
		for _, es := range m.pmt.ElementaryStreams {
			if es.ElementaryPID == m.pmt.PCRPID {
				hasPCRPID = true
				break
			}
		}
		if !hasPCRPID {
			return ErrPCRPIDInvalid
		}

		psiData := psi.Data{
			Sections: []psi.Section{
				{
					Header: psi.SectionHeader{
						SectionLength:          uint16(m.pmt.CalcSectionLength()),
						SectionSyntaxIndicator: true,
						TableID:                psi.TableIDPMT,
					},
					Syntax: &psi.SectionSyntax{
						Data: &m.pmt,
						Header: psi.SectionSyntaxHeader{
							CurrentNextIndicator: true,
							//LastSectionNumber:    0,
							//SectionNumber:        0,
							TableIDExtension: m.pmt.ProgramNumber,
							VersionNumber:    uint8(m.pmtVersion.inc()),
						},
					},
				},
			},
		}

		if m.pmtData, err = psiData.Append(m.pmtData[:0]); err != nil {
			return
		}

		m.pmtUpdated = false
	}

	m.pmtBytes.Reset()

	l := len(m.pmtData)
	for i := 0; i <= l/packetMaxPayload; i++ {
		start := i * packetMaxPayload
		stop := start + packetMaxPayload
		if stop > l {
			stop = l
		}
		pkt := ts.Packet{
			Header: ts.PacketHeader{
				HasPayload:                true,
				PayloadUnitStartIndicator: i == 0,
				PID:                       pmtStartPID, // FIXME multiple programs support
				ContinuityCounter:         uint8(m.pmtCC.inc()),
			},
			Payload: m.pmtData[start:stop],
		}
		if _, err = pkt.Put(m.pkt); err != nil {
			return
		}
		m.pmtBytes.Write(m.pkt)
	}

	return
}

func toPATData(pm *pidmap.Map[uint16]) *psi.PAT {
	d := &psi.PAT{
		Programs:          make([]psi.PATProgram, 0, len(pm.Keys)),
		TransportStreamID: uint16(psi.TableIDPAT),
	}

	for i, pid := range pm.Keys {
		d.Programs = append(d.Programs, psi.PATProgram{
			ProgramMapID:  pid,
			ProgramNumber: pm.Vals[i],
		})
	}

	return d
}
