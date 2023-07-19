package astits

import (
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
	EIT             *EITData
	NIT             *NITData
	PAT             *PATData
	PES             *PESData
	PMT             *PMTData
	SDT             *SDTData
	TOT             *TOTData
	AdaptationField *PacketAdaptationField

	internalData *payload

	PID uint16
}

func (d *DemuxerData) Close() {
	if d.internalData != nil {
		poolOfPayload.put(d.internalData)
	}
}

// MuxerData represents a data to be written by Muxer
type MuxerData struct {
	PID             uint16
	AdaptationField *PacketAdaptationField
	PES             *PESData
}

// parseData parses a payload spanning over multiple packets and returns a set of data
func parseData(pl *PacketList, prs PacketsParser, pm *programMap) (ds []*DemuxerData, err error) {
	defer pl.Clear()
	// Use custom parser first
	if prs != nil {
		var skip bool
		if ds, skip, err = prs(pl); err != nil {
			err = fmt.Errorf("astits: custom packets parsing failed: %w", err)
			return
		} else if skip {
			return
		}
		pl.IteratorReset()
	}

	// Get the slice for payload from pool
	p := poolOfPayload.get(pl.GetSize())

	// Append payload
	var c int
	for p := pl.IteratorGet(); p != nil; p = pl.IteratorNext() {
		c += copy(p.bs[c:], p.Payload)
	}

	// Create reader
	i := astikit.NewBytesIterator(p.bs)

	fp := pl.GetHead()
	pid := fp.Header.PID
	af := fp.AdaptationField

	// Parse payload
	if pid == PIDCAT {
		// Information in a CAT payload is private and dependent on the CA system. Use the PacketsParser
		// to parse this type of payload
	} else if isPSIPayload(pid, pm) {
		// Parse PSI data
		var psiData *PSIData
		if psiData, err = parsePSIData(i); err != nil {
			err = fmt.Errorf("astits: parsing PSI data failed: %w", err)
			return
		}

		// Append data
		ds = psiData.toData(af, pid)
	} else if isPESPayload(p.bs) {
		// Parse PES data
		pesData := &PESData{}
		if err = pesData.parsePESData(i); err != nil {
			err = fmt.Errorf("astits: parsing PES data failed: %w", err)
			return
		}

		// Append data
		ds = []*DemuxerData{{
			AdaptationField: af,
			PES:             pesData,
			PID:             pid,

			internalData: p,
		}}
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
	defer pl.IteratorReset()
	// Get the slice for payload from pool
	p := poolOfPayload.get(pl.GetSize())
	defer poolOfPayload.put(p)

	// Append payload
	var o int
	for p := pl.IteratorGet(); p != nil; p = pl.IteratorNext() {
		o += copy(p.bs[o:], p.Payload)
	}

	// Create reader
	i := astikit.NewBytesIterator(p.bs)

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
