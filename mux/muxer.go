package mux

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/asticode/go-astikit"

	"github.com/k-danil/go-astits/internal/pidmap"
	"github.com/k-danil/go-astits/internal/programmap"
	"github.com/k-danil/go-astits/pes"
	"github.com/k-danil/go-astits/psi"
	"github.com/k-danil/go-astits/ts"
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

type Muxer struct {
	ctx context.Context
	w   io.Writer

	packetSize             int
	tablesRetransmitPeriod int // period in PES packets

	pm         *programmap.Map // pid -> programNumber
	pmt        psi.PMTData
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
	pes     []byte
	stuffAF ts.PacketAdaptationField

	patDataBytes bytes.Buffer
	pmtDataBytes bytes.Buffer

	esContexts              pidmap.Map[esContext]
	tablesRetransmitCounter int
}

type esContext struct {
	es *psi.PMTElementaryStream
	cc wrappingCounter
}

func MuxerOptTablesRetransmitPeriod(newPeriod int) func(*Muxer) {
	return func(m *Muxer) {
		m.tablesRetransmitPeriod = newPeriod
	}
}

// TODO MuxerOptAutodetectPCRPID selecting first video PID for each PMT, falling back to first audio, falling back to any other

func NewMuxer(ctx context.Context, w io.Writer, opts ...func(*Muxer)) (m *Muxer) {
	m = &Muxer{
		ctx: ctx,
		w:   w,

		packetSize:             ts.MpegTsPacketSize, // no 192-byte packet support yet
		tablesRetransmitPeriod: 40,

		pm: programmap.New(),
		pmt: psi.PMTData{
			ElementaryStreams: []psi.PMTElementaryStream{},
			ProgramNumber:     programNumberStart,
		},

		// table version is 5-bit field
		patVersion: newWrappingCounter(0b11111),
		pmtVersion: newWrappingCounter(0b11111),

		patCC: newWrappingCounter(0b1111),
		pmtCC: newWrappingCounter(0b1111),
	}

	m.pkt = make([]byte, m.packetSize)
	m.pes = make([]byte, m.packetSize)

	// TODO multiple programs support
	m.pm.SetUnlocked(pmtStartPID, programNumberStart)
	m.pmUpdated = true

	for _, opt := range opts {
		opt(m)
	}

	// to output tables at the very start
	m.tablesRetransmitCounter = m.tablesRetransmitPeriod

	return
}

// if es.ElementaryPID is zero, it will be generated automatically
func (m *Muxer) AddElementaryStream(es psi.PMTElementaryStream) error {
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

func (m *Muxer) SetCC(pid uint16, cc uint8) error {
	ctx := m.esContexts.Get(pid)
	if ctx == nil {
		return ErrPIDNotFound
	}
	return ctx.cc.set(int(cc))
}

// WriteData writes MuxerData to TS stream
// Currently only PES packets are supported
// Be aware that after successful call WriteData will set d.AdaptationField.StuffingLength value to zero
func (m *Muxer) WriteData(d *MuxerData) (bytesWritten int, err error) {
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
		pktLen := ts.MpegTsPacketHeaderSize
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
			pktLen += 1 + int(d.AdaptationField.CalcLength())
			writeAf = false
		}

		bytesAvailable := m.packetSize - pktLen
		if payloadStart {
			pesHeaderLengthCurrent := pes.HeaderLength + int(d.PES.Header.OptionalHeader.CalcLength())
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

			var ntot, npayload int
			if ntot, npayload, err = d.PES.Header.PutPESData(
				m.pes[:bytesAvailable],
				d.PES.Data[payloadBytesWritten:],
				payloadStart,
			); err != nil {
				return
			}

			payloadBytesWritten += npayload

			pkt.Payload = m.pes[:ntot]

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

			if n, err = pkt.WriteBytes(m.pkt, m.w); err != nil {
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
	return p.WriteBytes(m.pkt, m.w)
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

func (m *Muxer) generatePAT() (err error) {
	if m.pmUpdated {
		d := toPATData(m.pm)

		psiData := psi.PSIData{
			Sections: []psi.PSISection{
				{
					Header: psi.PSISectionHeader{
						SectionLength:          d.CalcPATSectionLength(),
						SectionSyntaxIndicator: true,
						TableID:                psi.PSITableID(d.TransportStreamID),
					},
					Syntax: &psi.PSISectionSyntax{
						Data: d,
						Header: psi.PSISectionSyntaxHeader{
							CurrentNextIndicator: true,
							// TODO support for PAT tables longer than 1 TS packet
							//LastSectionNumber:    0,
							//SectionNumber:        0,
							TableIDExtension: d.TransportStreamID,
							VersionNumber:    uint8(m.patVersion.inc()),
						},
					},
				},
			},
		}

		m.patDataBytes.Reset()
		w := astikit.NewBitsWriter(astikit.BitsWriterOptions{Writer: &m.patDataBytes})
		if _, err = psiData.WritePSIData(w); err != nil {
			return
		}

		m.pmUpdated = false
	}

	m.patBytes.Reset()
	pkt := ts.Packet{
		Header: ts.PacketHeader{
			HasPayload:                true,
			PayloadUnitStartIndicator: true,
			PID:                       ts.PIDPAT,
			ContinuityCounter:         uint8(m.patCC.inc()),
		},
		Payload: m.patDataBytes.Bytes(),
	}
	if _, err = pkt.WriteBytes(m.pkt, &m.patBytes); err != nil {
		// FIXME save old PAT and rollback to it here maybe?
		return
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

		psiData := psi.PSIData{
			Sections: []psi.PSISection{
				{
					Header: psi.PSISectionHeader{
						SectionLength:          m.pmt.CalcPMTSectionLength(),
						SectionSyntaxIndicator: true,
						TableID:                psi.PSITableIDPMT,
					},
					Syntax: &psi.PSISectionSyntax{
						Data: &m.pmt,
						Header: psi.PSISectionSyntaxHeader{
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

		m.pmtDataBytes.Reset()
		w := astikit.NewBitsWriter(astikit.BitsWriterOptions{Writer: &m.pmtDataBytes})
		if _, err = psiData.WritePSIData(w); err != nil {
			return
		}

		m.pmtUpdated = false
	}

	m.pmtBytes.Reset()

	l := m.pmtDataBytes.Len()
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
			Payload: m.pmtDataBytes.Bytes()[start:stop],
		}
		if _, err = pkt.WriteBytes(m.pkt, &m.pmtBytes); err != nil {
			// FIXME save old PMT and rollback to it here maybe?
			return
		}
	}

	return
}

func toPATData(pm *programmap.Map) *psi.PATData {
	d := &psi.PATData{
		Programs:          make([]psi.PATProgram, 0, len(pm.P)),
		TransportStreamID: uint16(psi.PSITableIDPAT),
	}

	for pid, pnr := range pm.P {
		d.Programs = append(d.Programs, psi.PATProgram{
			ProgramMapID:  uint16(pid),
			ProgramNumber: pnr,
		})
	}

	return d
}
