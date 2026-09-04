package mux

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/k-danil/go-astits/v3/internal/pidmap"
	"github.com/k-danil/go-astits/v3/pes"
	"github.com/k-danil/go-astits/v3/psi"
	"github.com/k-danil/go-astits/v3/ts"
)

const (
	pmtStartPID        uint16 = 0x1000
	programNumberStart uint16 = 1
	packetMaxPayload          = 184
	// A PES header can exceed one packet, so it is serialized once and spanned.
	maxPESHeader = pes.HeaderSize + 3 + 0xff

	tableVersionWrap = 0b11111
	ccWrap           = 0b1111
	afLengthByte     = 1
	afFlagsByte      = 1

	reservedPIDLast uint16 = 0x1f
)

var (
	ErrPIDNotFound            = errors.New("astits: PID not found")
	ErrPIDAlreadyExists       = errors.New("astits: PID already exists")
	ErrPCRPIDInvalid          = errors.New("astits: PCR PID invalid")
	ErrReservedPID            = errors.New("astits: PID reserved for tables or stuffing")
	ErrAdaptationFieldTooLong = errors.New("astits: adaptation field leaves no room in the packet")
)

// Muxer writes an MPEG-TS stream for a single program.
type Muxer struct {
	ctx context.Context
	w   io.Writer

	packetSize             int
	tablesRetransmitPeriod int

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

	pkt       []byte
	pesHdr    []byte
	stuffAF   ts.PacketAdaptationField
	pktArr    [ts.PacketSize]byte
	pesHdrArr [maxPESHeader]byte

	patData []byte
	pmtData []byte

	esContexts              pidmap.Map[esContext]
	tablesRetransmitCounter int

	pmKeysArr [4]uint16
	pmValsArr [4]uint16
	esKeysArr [8]uint16
	esValsArr [8]esContext
	patArr    [ts.PacketSize]byte
	pmtArr    [ts.PacketSize]byte
}

type esContext struct {
	es *psi.ElementaryStream
	cc wrappingCounter
}

// Period is counted in written PES packets (units), not TS packets.
func WithTablesRetransmitPeriod(newPeriod int) func(*Muxer) {
	return func(m *Muxer) {
		m.tablesRetransmitPeriod = newPeriod
	}
}

func New(ctx context.Context, w io.Writer, opts ...func(*Muxer)) (m *Muxer) {
	m = &Muxer{
		ctx: ctx,
		w:   w,

		packetSize:             ts.PacketSize,
		tablesRetransmitPeriod: 40,

		pmt: psi.PMT{
			ElementaryStreams: []psi.ElementaryStream{},
			ProgramNumber:     programNumberStart,
		},

		patVersion: newWrappingCounter(tableVersionWrap),
		pmtVersion: newWrappingCounter(tableVersionWrap),

		patCC: newWrappingCounter(0b1111),
		pmtCC: newWrappingCounter(0b1111),

		nextPID: pmtStartPID + 1,
	}

	m.pkt = m.pktArr[:]
	m.pesHdr = m.pesHdrArr[:]
	m.pm = pidmap.Map[uint16]{Keys: m.pmKeysArr[:0], Vals: m.pmValsArr[:0]}
	m.esContexts = pidmap.Map[esContext]{Keys: m.esKeysArr[:0], Vals: m.esValsArr[:0]}
	m.patData = m.patArr[:0]
	m.pmtData = m.pmtArr[:0]

	m.pm.Set(pmtStartPID, programNumberStart)
	m.pmUpdated = true

	for _, opt := range opts {
		opt(m)
	}

	// start at the period so the first WriteData emits the tables
	m.tablesRetransmitCounter = m.tablesRetransmitPeriod

	return
}

// if es.ElementaryPID is zero, it will be generated automatically
func (m *Muxer) AddElementaryStream(es psi.ElementaryStream) (err error) {
	if es.ElementaryPID != 0 {
		if reservedPID(es.ElementaryPID) {
			return ErrReservedPID
		}
		if m.esContexts.Get(es.ElementaryPID) != nil {
			return ErrPIDAlreadyExists
		}
	} else {
		for m.esContexts.Get(m.nextPID) != nil {
			m.nextPID++
		}
		if reservedPID(m.nextPID) {
			return ErrReservedPID
		}
		es.ElementaryPID = m.nextPID
		m.nextPID++
	}

	m.pmt.ElementaryStreams = append(m.pmt.ElementaryStreams, es)

	*m.esContexts.GetOrAdd(es.ElementaryPID) = esContext{
		es: &es,
		cc: newWrappingCounter(ccWrap),
	}
	m.pmtUpdated = true
	return nil
}

func reservedPID(pid uint16) bool {
	return pid <= reservedPIDLast || pid == pmtStartPID || pid >= ts.PIDNull
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
	m.pmtUpdated = true
	return nil
}

func (m *Muxer) SetPCRPID(pid uint16) {
	m.pmt.PCRPID = pid
	m.pmtUpdated = true
}

func (m *Muxer) SetCC(pid uint16, cc uint8) error {
	ctx := m.esContexts.Get(pid)
	if ctx == nil {
		return ErrPIDNotFound
	}
	return ctx.cc.set(int(cc))
}

// Always overwrites d.AdaptationField.StuffingLength and IsOneByteStuffing: a caller's value would be counted twice. Issues several writes per unit, so buffer an unbuffered destination.
func (m *Muxer) WriteData(d *Data) (bytesWritten int, err error) {
	ctx := m.esContexts.Get(d.PID)
	if ctx == nil {
		return 0, ErrPIDNotFound
	}

	afLen := 0
	if d.AdaptationField != nil {
		d.AdaptationField.StuffingLength, d.AdaptationField.IsOneByteStuffing = 0, false
		if afLen = afLengthByte + d.AdaptationField.CalcLength(); afLen > packetMaxPayload {
			return 0, ErrAdaptationFieldTooLong
		}
	}

	forceTables := d.AdaptationField != nil &&
		d.AdaptationField.RandomAccessIndicator &&
		d.PID == m.pmt.PCRPID

	var n int
	if n, err = m.retransmitTables(forceTables); err != nil {
		return n, err
	}

	bytesWritten += n

	if d.PES.Header.StreamID == 0 {
		d.PES.Header.StreamID = ctx.es.StreamType.ToPESStreamID()
	}

	var hdrLen int
	if hdrLen, err = d.PES.Header.PutHeader(m.pesHdr, len(d.PES.Data)); err != nil {
		return
	}
	pesHdr := m.pesHdr[:hdrLen]

	bulkChunk := m.packetSize - ts.HeaderSize
	firstPktLen := ts.HeaderSize + afLen

	// Ends on a packet boundary either way, so the bulk and tail phases below can finish the unit.
	payloadWritten := 0
	if firstAvail := m.packetSize - firstPktLen; hdrLen <= firstAvail {
		firstPayload := min(len(d.PES.Data), firstAvail-hdrLen)
		content := hdrLen + firstPayload
		header := ts.PacketHeader{
			ContinuityCounter:         uint8(ctx.cc.inc()),
			PID:                       d.PID,
			HasPayload:                true,
			PayloadUnitStartIndicator: true,
		}
		var af *ts.PacketAdaptationField
		if stuffing := firstAvail - content; d.AdaptationField != nil {
			header.HasAdaptationField = true
			af = d.AdaptationField
			af.StuffingLength = uint8(stuffing)
		} else if stuffing > 0 {
			header.HasAdaptationField = true
			af = m.stuffingAdaptationField(stuffing)
		}
		if n, err = m.emitPacket(header, af, m.packetSize-content, pesHdr, d.PES.Data[:firstPayload]); err != nil {
			return
		}
		bytesWritten += n
		payloadWritten = firstPayload
	} else {
		writeAf := d.AdaptationField != nil
		for hdrWritten := 0; hdrWritten < hdrLen; {
			header := ts.PacketHeader{ContinuityCounter: uint8(ctx.cc.value), PID: d.PID}
			var af *ts.PacketAdaptationField
			pktLen := ts.HeaderSize
			if writeAf {
				header.HasAdaptationField = true
				af = d.AdaptationField
				pktLen += afLengthByte + d.AdaptationField.CalcLength()
				writeAf = false
			}
			bytesAvailable := m.packetSize - pktLen
			hdrChunk := min(hdrLen-hdrWritten, bytesAvailable)
			payloadChunk := min(len(d.PES.Data)-payloadWritten, bytesAvailable-hdrChunk)
			content := hdrChunk + payloadChunk
			// H.222.0 2.4.3.3: an adaptation-field-only packet repeats the counter instead of advancing it.
			if content > 0 {
				header.HasPayload = true
				header.ContinuityCounter = uint8(ctx.cc.inc())
				if hdrWritten == 0 {
					header.PayloadUnitStartIndicator = true
				}
			}
			// Stuff the leftover through the adaptation field: a PES of unspecified
			// length would read trailing 0xff padding as payload.
			if stuffing := bytesAvailable - content; stuffing > 0 {
				header.HasAdaptationField = true
				if af == nil {
					af = m.stuffingAdaptationField(stuffing)
				} else {
					af.StuffingLength = uint8(stuffing)
				}
			}
			if n, err = m.emitPacket(header, af, m.packetSize-content,
				pesHdr[hdrWritten:hdrWritten+hdrChunk],
				d.PES.Data[payloadWritten:payloadWritten+payloadChunk]); err != nil {
				return
			}
			bytesWritten += n
			hdrWritten += hdrChunk
			payloadWritten += payloadChunk
		}
	}

	// m.pkt holds this run's header while fastLocked: nothing else may write into it, only the CC is patched.
	fastHeader := ts.PacketHeader{PID: d.PID, HasPayload: true}
	fastLocked := false
	for len(d.PES.Data)-payloadWritten >= bulkChunk {
		cc := uint8(ctx.cc.inc())
		if fastLocked {
			ts.SetContinuityCounter(m.pkt, cc)
		} else {
			fastHeader.ContinuityCounter = cc
			fastHeader.Put(m.pkt)
			fastLocked = true
		}
		if n, err = m.w.Write(m.pkt[:ts.HeaderSize]); err != nil {
			return
		}
		bytesWritten += n
		if n, err = m.w.Write(d.PES.Data[payloadWritten : payloadWritten+bulkChunk]); err != nil {
			return
		}
		bytesWritten += n
		payloadWritten += bulkChunk
	}

	if rem := len(d.PES.Data) - payloadWritten; rem > 0 {
		header := ts.PacketHeader{
			ContinuityCounter:  uint8(ctx.cc.inc()),
			PID:                d.PID,
			HasPayload:         true,
			HasAdaptationField: true,
		}
		if n, err = m.emitPacket(header, m.stuffingAdaptationField(bulkChunk-rem),
			m.packetSize-rem, nil, d.PES.Data[payloadWritten:]); err != nil {
			return
		}
		bytesWritten += n
	}

	return
}

// front is how many bytes of m.pkt (header, AF, stuffing) precede hdr and payload.
func (m *Muxer) emitPacket(header ts.PacketHeader, af *ts.PacketAdaptationField, front int, hdr, payload []byte) (n int, err error) {
	header.Put(m.pkt)
	if header.HasAdaptationField {
		if _, err = af.Put(m.pkt[ts.HeaderSize:]); err != nil {
			return
		}
	}
	var w int
	if w, err = m.w.Write(m.pkt[:front]); err != nil {
		return
	}
	n = w
	if len(hdr) > 0 {
		if w, err = m.w.Write(hdr); err != nil {
			return
		}
		n += w
	}
	if len(payload) > 0 {
		if w, err = m.w.Write(payload); err != nil {
			return
		}
		n += w
	}
	return
}

func (m *Muxer) WritePacket(p *ts.Packet) (int, error) {
	if raw := p.Raw(); len(raw) > 0 {
		return m.w.Write(raw)
	}
	if _, err := p.Put(m.pkt); err != nil {
		return 0, err
	}
	return m.w.Write(m.pkt)
}

// Returns the shared scratch AF: the previous one is invalidated.
func (m *Muxer) stuffingAdaptationField(bytesToStuff int) *ts.PacketAdaptationField {
	m.stuffAF.Reset()
	if bytesToStuff == 1 {
		m.stuffAF.IsOneByteStuffing = true
	} else {
		m.stuffAF.StuffingLength = uint8(bytesToStuff - afLengthByte - afFlagsByte)
	}
	return &m.stuffAF
}

func (m *Muxer) retransmitTables(force bool) (n int, err error) {
	m.tablesRetransmitCounter++
	if !force && m.tablesRetransmitCounter < m.tablesRetransmitPeriod {
		return
	}

	return m.WriteTables()
}

func (m *Muxer) WriteTables() (bytesWritten int, err error) {
	// A counter burned on a table that never reached the wire shows up as a continuity gap at the decoder.
	patCC, pmtCC := m.patCC, m.pmtCC
	defer func() {
		if err != nil {
			m.patCC, m.pmtCC = patCC, pmtCC
		}
	}()

	if err = m.validatePCRPID(); err != nil {
		return
	}

	if err = m.generatePAT(); err != nil {
		return
	}

	if err = m.generatePMT(); err != nil {
		return
	}

	patchContinuityCounters(m.patBytes.Bytes(), &m.patCC)
	patchContinuityCounters(m.pmtBytes.Bytes(), &m.pmtCC)

	var n int
	if n, err = m.w.Write(m.patBytes.Bytes()); err != nil {
		return
	}
	bytesWritten += n

	if n, err = m.w.Write(m.pmtBytes.Bytes()); err != nil {
		return
	}
	bytesWritten += n

	m.tablesRetransmitCounter = 0
	return
}

func patchContinuityCounters(bs []byte, cc *wrappingCounter) {
	for off := 0; off < len(bs); off += ts.PacketSize {
		ts.SetContinuityCounter(bs[off:], uint8(cc.inc()))
	}
}

func (m *Muxer) validatePCRPID() (err error) {
	if !m.pmtUpdated {
		return
	}
	for _, es := range m.pmt.ElementaryStreams {
		if es.ElementaryPID == m.pmt.PCRPID {
			return
		}
	}
	return ErrPCRPIDInvalid
}

const (
	psiSectionBodyMax        = 1021
	psiSyntaxHeaderLen       = 5
	crc32Len                 = 4
	patProgramSize           = 4
	maxPATProgramsPerSection = (psiSectionBodyMax - psiSyntaxHeaderLen - crc32Len) / patProgramSize
)

func (m *Muxer) generatePAT() (err error) {
	if m.pmUpdated {
		d := toPATData(&m.pm)

		numSections := (len(d.Programs) + maxPATProgramsPerSection - 1) / maxPATProgramsPerSection
		if numSections == 0 {
			numSections = 1
		}
		version := uint8(m.patVersion.inc())

		psiData := psi.Data{Sections: make([]psi.Section, 0, numSections)}
		for si := range numSections {
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

		m.patBytes.Reset()
		l := len(m.patData)
		for i := 0; i*packetMaxPayload < l; i++ {
			start := i * packetMaxPayload
			stop := min(start+packetMaxPayload, l)
			pkt := ts.Packet{
				Header: ts.PacketHeader{
					HasPayload:                true,
					PayloadUnitStartIndicator: i == 0,
					PID:                       ts.PIDPAT,
				},
				Payload: m.patData[start:stop],
			}
			if _, err = pkt.Put(m.pkt); err != nil {
				return
			}
			m.patBytes.Write(m.pkt)
		}
	}

	return
}

func (m *Muxer) generatePMT() (err error) {
	if m.pmtUpdated {
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
							TableIDExtension:     m.pmt.ProgramNumber,
							VersionNumber:        uint8(m.pmtVersion.inc()),
						},
					},
				},
			},
		}

		if m.pmtData, err = psiData.Append(m.pmtData[:0]); err != nil {
			return
		}

		m.pmtUpdated = false

		m.pmtBytes.Reset()
		l := len(m.pmtData)
		for i := 0; i*packetMaxPayload < l; i++ {
			start := i * packetMaxPayload
			stop := min(start+packetMaxPayload, l)
			pkt := ts.Packet{
				Header: ts.PacketHeader{
					HasPayload:                true,
					PayloadUnitStartIndicator: i == 0,
					PID:                       pmtStartPID,
				},
				Payload: m.pmtData[start:stop],
			}
			if _, err = pkt.Put(m.pkt); err != nil {
				return
			}
			m.pmtBytes.Write(m.pkt)
		}
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
