package astits

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/asticode/go-astikit"
)

// PIDs
const (
	PIDPAT  uint16 = 0x0    // Program Association Table (PAT) contains a directory listing of all Program Map Tables.
	PIDCAT  uint16 = 0x1    // Conditional Access Table (CAT) contains a directory listing of all ITU-T Rec. H.222 entitlement management message streams used by Program Map Tables.
	PIDTSDT uint16 = 0x2    // Transport Stream Description Table (TSDT) contains descriptors related to the overall transport stream
	PIDNull uint16 = 0x1fff // Null Packet (used for fixed bandwidth padding)
)

// DemuxerData represents a data parsed by Demuxer
type DemuxerData struct {
	PAT *PATData
	PMT *PMTData
	PES *PESData
	EIT *EITData
	NIT *NITData
	SDT *SDTData
	TOT *TOTData

	AdaptationField *PacketAdaptationField
	af              PacketAdaptationField
	pes             PESData

	internalData *dataPayload

	ContinuityCounter uint8
	PID               uint16
}

func (d *DemuxerData) Close() {
	if d.internalData != nil {
		poolOfPayload.put(d.internalData)
	}
}

// setAdaptationField stores an OWNED copy: the source AF lives in the embedded
// struct of a pooled packet that is reused right after parseData.
func (d *DemuxerData) setAdaptationField(src *PacketAdaptationField) {
	if src == nil {
		return
	}
	d.af = *src
	if src.TransportPrivateData != nil {
		d.af.TransportPrivateData = d.af.privBuf[:len(src.TransportPrivateData)]
	}
	d.AdaptationField = &d.af
}

// MuxerData represents a data to be written by Muxer
type MuxerData struct {
	PID             uint16
	AdaptationField *PacketAdaptationField
	PES             *PESData
}

// parseData parses a payload spanning over multiple packets and returns a set of data
func parseData(pl *PacketList, prs PacketsParser, pm *programMap, psiPrev *pidMap[[]byte], scratch []*DemuxerData) (ds []*DemuxerData, err error) {
	// Use custom parser first
	if prs != nil {
		var skip bool
		if ds, skip, err = prs(pl); err != nil {
			err = fmt.Errorf("astits: custom packets parsing failed: %w", err)
			return
		} else if skip {
			return
		}
	}

	// Get the slice for payload from pool
	dp := poolOfPayload.get(pl.Size())

	// Append payload
	var c int
	for p := pl.Head(); p != nil; p = p.Next() {
		c += copy(dp.bs[c:], p.Payload)
	}

	fp := pl.Head()
	pid := fp.Header.PID
	af := fp.AdaptationField
	cc := fp.Header.ContinuityCounter

	// Parse payload
	switch {
	case pid == PIDCAT:
		// Information in a CAT payload is private and dependent on the CA system. Use the PacketsParser
		// to parse this type of payload
		poolOfPayload.put(dp)
	case isPSIPayload(pid, pm):
		// PSI repeat dedup: an identical section carries no new information — neither
		// parse nor emit. A suppressed dp is safe to return to the pool (unlike a parsed
		// one: descriptors keep views into dp.bs, hence no put after parsing).
		if psiPrev != nil {
			if prev := psiPrev.get(pid); prev != nil && bytes.Equal(*prev, dp.bs) {
				poolOfPayload.put(dp)
				return
			}
		}

		// Parse PSI data
		var psiData *PSIData
		if psiData, err = parsePSIData(astikit.NewBytesIterator(dp.bs)); err != nil {
			poolOfPayload.put(dp)
			err = fmt.Errorf("astits: parsing PSI data failed: %w", err)
			return
		}

		if psiPrev != nil {
			sec := psiPrev.getOrAdd(pid)
			*sec = append((*sec)[:0], dp.bs...)
		}

		// Append data
		ds = psiData.toData(af, pid)
		for _, d := range ds {
			d.setAdaptationField(af)
		}
	case isPESPayload(dp.bs):
		d := &DemuxerData{
			PID:               pid,
			ContinuityCounter: cc,

			internalData: dp,
		}

		// Parse PES data
		if err = d.pes.parsePESData(dp.bs); err != nil {
			poolOfPayload.put(dp)
			err = fmt.Errorf("astits: parsing PES data failed: %w", err)
			return
		}
		d.PES = &d.pes

		d.setAdaptationField(af)
		ds = append(scratch[:0], d)
	default:
		// Unknown payload: no data will be produced, return the buffer to the pool
		poolOfPayload.put(dp)
	}
	return
}

// isPSIPayload checks whether the payload is a PSI one
func isPSIPayload(pid uint16, pm *programMap) bool {
	return pid == PIDPAT || // PAT
		pm.existsUnlocked(pid) || // PMT
		((pid >= 0x10 && pid <= 0x14) || (pid >= 0x1e && pid <= 0x1f)) //DVB
}

// isPESPayload checks whether the payload is a PES one
func isPESPayload(bs []byte) bool {
	// Packet is not big enough
	if len(bs) < 4 {
		return false
	}

	// Check prefix
	return binary.BigEndian.Uint32(bs)>>8 == 1
}

// isPSIComplete checks whether we have sufficient amount of packets to parse PSI
func isPSIComplete(pl *PacketList) bool {
	// Get the slice for payload from pool
	dp := poolOfPayload.get(pl.Size())
	defer poolOfPayload.put(dp)

	// Append payload
	var o int
	for p := pl.Head(); p != nil; p = p.Next() {
		o += copy(dp.bs[o:], p.Payload)
	}

	// Create reader
	i := astikit.NewBytesIterator(dp.bs)

	// Get next byte
	b, err := i.NextByte()
	if err != nil {
		return false
	}

	// Pointer filler bytes
	i.Skip(int(b))

	for i.HasBytesLeft() {

		// Get PSI table ID
		b, err = i.NextByte()
		if err != nil {
			return false
		}

		// Check whether we need to stop the parsing
		if shouldStopPSIParsing(PSITableID(b)) {
			break
		}

		// Get PSI section length
		var bs []byte
		bs, err = i.NextBytesNoCopy(2)
		if err != nil {
			return false
		}

		i.Skip(int(binary.BigEndian.Uint16(bs) & 0x0fff))
	}

	return i.Len() >= i.Offset()
}
