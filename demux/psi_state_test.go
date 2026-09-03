package demux

import (
	"bytes"
	"context"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v3/psi"
	"github.com/k-danil/go-astits/v3/ts"
)

// payloadPacket frames payload as a packet, 0xFF-stuffed.
func payloadPacket(pid uint16, cc uint8, pusi bool, payload []byte) []byte {
	flags := byte(0)
	if pusi {
		flags = 0x40
	}
	p := []byte{syncByte, flags | byte(pid>>8), byte(pid), 0x10 | cc}
	p = append(p, payload...)
	return append(p, bytes.Repeat([]byte{0xff}, ts.PacketSize-len(p))...)
}

func psiPacket(pid uint16, cc uint8, section []byte) []byte {
	return payloadPacket(pid, cc, true, append([]byte{0x00}, section...))
}

func sectionWithCRC(id psi.TableID, body []byte) []byte {
	l := len(body) + 4
	s := append([]byte{byte(id), 0xb0 | byte(l>>8), byte(l)}, body...)
	return binary.BigEndian.AppendUint32(s, ts.ComputeCRC32(s))
}

func syntaxHeader(version uint8, current bool, section, last uint8) []byte {
	cn := byte(0)
	if current {
		cn = 1
	}
	return []byte{0x00, 0x01, 0xc0 | version<<1 | cn, section, last}
}

// patSectionAt is one PAT section mapping program to pmtPID.
func patSectionAt(version uint8, current bool, section, last uint8, program, pmtPID uint16) []byte {
	body := append(syntaxHeader(version, current, section, last),
		byte(program>>8), byte(program), 0xe0|byte(pmtPID>>8), byte(pmtPID))
	return sectionWithCRC(psi.TableIDPAT, body)
}

// patSection maps program 1 to pmtPID in a single-section PAT.
func patSection(version uint8, current bool, pmtPID uint16) []byte {
	return patSectionAt(version, current, 0, 0, 1, pmtPID)
}

// pmtSection lists one AVC stream on esPID.
func pmtSection(version uint8, current bool, esPID uint16) []byte {
	body := append(syntaxHeader(version, current, 0, 0),
		0xe0|byte(esPID>>8), byte(esPID), 0xf0, 0x00,
		0x1b, 0xe0|byte(esPID>>8), byte(esPID), 0xf0, 0x00)
	return sectionWithCRC(psi.TableIDPMT, body)
}

var unboundedPES = []byte{0x00, 0x00, 0x01, 0xe0, 0x00, 0x00, 0x80, 0x00, 0x00, 'p', 'e', 's'}

func TestDemuxerTableNotCurrentLeavesState(t *testing.T) {
	stream := psiPacket(ts.PIDPAT, 0, patSection(0, true, 0x100))
	stream = append(stream, psiPacket(ts.PIDPAT, 1, patSection(1, false, 0x200))...)
	stream = append(stream, psiPacket(0x200, 0, pmtSection(0, false, 0x201))...)
	dmx := New(context.Background(), bytes.NewReader(stream), WithPacketSize(ts.PacketSize), WithRecoverableErrors())

	ev, err := dmx.Next()
	require.NoError(t, err)
	assert.Equal(t, EventPAT, ev)

	ev, err = dmx.Next()
	require.NoError(t, err)
	assert.Equal(t, EventPAT, ev)
	_, sec := dmx.Section()
	assert.False(t, sec.Syntax.Header.CurrentNextIndicator)
	assert.Equal(t, uint8(1), sec.Syntax.Header.VersionNumber)
	announced, ok := sec.Syntax.Data.(*psi.PAT)
	require.True(t, ok)
	assert.Equal(t, uint16(0x200), announced.Programs[0].ProgramMapID)
	assert.Equal(t, uint16(0x100), dmx.PAT().Programs[0].ProgramMapID, "the announced PAT is not in effect")
	assert.Equal(t, []uint16{0x100}, dmx.programMap.Keys, "the announced layout stays out of the map in effect")
	assert.Equal(t, []uint16{0x200}, dmx.acc.nextMap.Keys, "it is kept apart, so its PMT still parses")

	ev, err = dmx.Next()
	require.NoError(t, err, "the announced PMT PID is parsed as PSI, not lost as an unknown unit")
	assert.Equal(t, EventPMT, ev)
	_, sec = dmx.Section()
	assert.False(t, sec.Syntax.Header.CurrentNextIndicator)
	assert.Nil(t, dmx.PMT())

	_, err = dmx.Next()
	assert.ErrorIs(t, err, ts.ErrNoMorePackets)
}

// An announced PAT must not take over a PID that carries a live stream.
func TestDemuxerNextPATLeavesLiveStream(t *testing.T) {
	const (
		pmtPID = uint16(0x100)
		esPID  = uint16(0x200)
	)
	stream := psiPacket(ts.PIDPAT, 0, patSection(0, true, pmtPID))
	stream = append(stream, psiPacket(pmtPID, 0, pmtSection(0, true, esPID))...)
	stream = append(stream, payloadPacket(esPID, 0, true, unboundedPES)...)
	stream = append(stream, psiPacket(ts.PIDPAT, 1, patSection(1, false, esPID))...)
	stream = append(stream, payloadPacket(esPID, 1, true, unboundedPES)...)
	stream = append(stream, payloadPacket(esPID, 2, true, unboundedPES)...)

	dmx := New(context.Background(), bytes.NewReader(stream), WithPacketSize(ts.PacketSize), WithRecoverableErrors())
	defer dmx.Close()

	var units int
	for ev, err := range dmx.Events() {
		require.NoError(t, err)
		if ev == EventPES {
			units++
			dmx.PES().Close()
		}
	}
	assert.Equal(t, 3, units, "the announcement leaves the stream on esPID alone")
}

func TestDemuxerProgramMapFollowsPATVersion(t *testing.T) {
	t.Run("a version bump frees the PID", func(t *testing.T) {
		stream := psiPacket(ts.PIDPAT, 0, patSection(0, true, 0x100))
		stream = append(stream, psiPacket(ts.PIDPAT, 1, patSection(1, true, 0x200))...)
		stream = append(stream, payloadPacket(0x100, 0, true, unboundedPES)...)
		dmx := New(context.Background(), bytes.NewReader(stream), WithPacketSize(ts.PacketSize))

		for range 2 {
			ev, err := dmx.Next()
			require.NoError(t, err)
			assert.Equal(t, EventPAT, ev)
		}
		assert.Equal(t, []uint16{0x200}, dmx.programMap.Keys)

		ev, err := dmx.Next()
		require.NoError(t, err)
		require.Equal(t, EventPES, ev, "the PID the new PAT freed carries PES again")
		assert.Equal(t, uint16(0x100), dmx.PES().PID)
	})

	t.Run("sections of one version add up in any order", func(t *testing.T) {
		stream := psiPacket(ts.PIDPAT, 0, patSectionAt(1, true, 1, 1, 2, 0x200))
		stream = append(stream, psiPacket(ts.PIDPAT, 1, patSectionAt(1, true, 0, 1, 1, 0x100))...)
		dmx := New(context.Background(), bytes.NewReader(stream), WithPacketSize(ts.PacketSize))

		for range 2 {
			ev, err := dmx.Next()
			require.NoError(t, err)
			assert.Equal(t, EventPAT, ev)
		}
		assert.ElementsMatch(t, []uint16{0x200, 0x100}, dmx.programMap.Keys)
	})
}

func TestDemuxerRepeatedBrokenSectionReported(t *testing.T) {
	broken := patSection(0, true, 0x100)
	broken[len(broken)-1] ^= 0x01
	stream := psiPacket(ts.PIDPAT, 0, broken)
	stream = append(stream, psiPacket(ts.PIDPAT, 1, broken)...)
	dmx := New(context.Background(), bytes.NewReader(stream), WithPacketSize(ts.PacketSize), WithRecoverableErrors())

	var crcErrors int
	for ev, err := range dmx.Events() {
		require.Error(t, err)
		assert.Equal(t, EventError, ev)
		var re *ts.RecoverableError
		require.ErrorAs(t, err, &re)
		assert.Equal(t, ts.ErrorKindCRC, re.Kind)
		assert.Equal(t, int64(len(broken)), re.Dropped)
		crcErrors++
	}
	assert.Equal(t, 2, crcErrors, "each occurrence of the damaged section is an event")
}
