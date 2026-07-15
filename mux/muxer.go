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
	pmtStartPID        uint16 = 0x1000
	programNumberStart uint16 = 1
	packetMaxPayload          = 184
	// Widest PES header: 6-byte prefix + optional header (2 flag bytes + a 1-byte
	// length + up to 255 data bytes). It can exceed a packet, so it is serialized
	// once and spanned across packets.
	maxPESHeader = pes.HeaderSize + 3 + 0xff
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

	pkt       []byte
	pesHdr    []byte // serialized PES header, spanned across packets
	stuffAF   ts.PacketAdaptationField
	pktArr    [ts.PacketSize]byte
	pesHdrArr [maxPESHeader]byte

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
	m.pesHdr = m.pesHdrArr[:]
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
// It issues several writes per unit (header and payload separately for full mid-unit
// packets), so wrap an unbuffered destination such as a raw file or socket in bufio.
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

	if d.PES.Header.StreamID == 0 {
		d.PES.Header.StreamID = ctx.es.StreamType.ToPESStreamID()
	}

	// Serialize the PES header once. Header and payload form one byte stream that
	// is split across packets; a header wider than a packet spans several of them.
	var hdrLen int
	if hdrLen, err = d.PES.Header.PutHeader(m.pesHdr, len(d.PES.Data)); err != nil {
		return
	}
	pesHdr := m.pesHdr[:hdrLen]

	bulkChunk := m.packetSize - ts.HeaderSize
	firstPktLen := ts.HeaderSize
	if d.AdaptationField != nil {
		firstPktLen += 1 + d.AdaptationField.CalcLength()
	}

	// Emit the PES header, then drain the payload. It usually fits the first
	// packet; one too wide (a fat AF ate the room) spans several. Either way this
	// ends on a packet boundary, so the shared bulk and tail phases below finish it.
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
			header := ts.PacketHeader{ContinuityCounter: uint8(ctx.cc.inc()), PID: d.PID}
			var af *ts.PacketAdaptationField
			pktLen := ts.HeaderSize
			if writeAf {
				header.HasAdaptationField = true
				af = d.AdaptationField
				// one byte for the adaptation field length field
				pktLen += 1 + d.AdaptationField.CalcLength()
				writeAf = false
			}
			bytesAvailable := m.packetSize - pktLen
			hdrChunk := min(hdrLen-hdrWritten, bytesAvailable)
			payloadChunk := min(len(d.PES.Data)-payloadWritten, bytesAvailable-hdrChunk)
			content := hdrChunk + payloadChunk
			if content > 0 {
				header.HasPayload = true
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

	// Bulk phase: full mid-unit packets — a fixed 4-byte header (only CC
	// advancing) and a packet-sized payload chunk, no PES header or AF. Between
	// them only CC changes, so it is patched in place instead of re-encoded.
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

	if d.AdaptationField != nil {
		d.AdaptationField.StuffingLength = 0
	}
	return
}

// emitPacket serializes header and af into the front of m.pkt, writes that front,
// then hdr and payload straight from their own buffers — like the bulk path, so
// neither is copied into m.pkt first.
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
				},
				Payload: m.patData[start:stop],
			}
			if _, err = pkt.Put(m.pkt); err != nil {
				return
			}
			m.patBytes.Write(m.pkt)
		}
	}

	// Only the continuity counter changes between emissions: patch it in place
	// instead of repacketizing (mirrors the PES fast path).
	b := m.patBytes.Bytes()
	for off := 0; off < len(b); off += ts.PacketSize {
		ts.SetContinuityCounter(b[off:], uint8(m.patCC.inc()))
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

		m.pmtBytes.Reset()
		l := len(m.pmtData)
		for i := 0; i <= l/packetMaxPayload; i++ {
			start := i * packetMaxPayload
			stop := min(start+packetMaxPayload, l)
			pkt := ts.Packet{
				Header: ts.PacketHeader{
					HasPayload:                true,
					PayloadUnitStartIndicator: i == 0,
					PID:                       pmtStartPID, // FIXME multiple programs support
				},
				Payload: m.pmtData[start:stop],
			}
			if _, err = pkt.Put(m.pkt); err != nil {
				return
			}
			m.pmtBytes.Write(m.pkt)
		}
	}

	// Only the continuity counter changes between emissions: patch it in place
	// instead of repacketizing (mirrors the PES fast path).
	b := m.pmtBytes.Bytes()
	for off := 0; off < len(b); off += ts.PacketSize {
		ts.SetContinuityCounter(b[off:], uint8(m.pmtCC.inc()))
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
