package demux

import (
	"bytes"
	"context"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v2/psi"
	"github.com/k-danil/go-astits/v2/ts"
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
	assert.Equal(t, uint16(0x200), sec.Syntax.Data.(*psi.PAT).Programs[0].ProgramMapID)
	assert.Equal(t, uint16(0x100), dmx.PAT().Programs[0].ProgramMapID, "the announced PAT is not in effect")
	assert.ElementsMatch(t, []uint16{0x100, 0x200}, dmx.programMap.Keys, "both layouts are PSI while one is announced")

	ev, err = dmx.Next()
	require.NoError(t, err, "the announced PMT PID is parsed as PSI, not lost as an unknown unit")
	assert.Equal(t, EventPMT, ev)
	_, sec = dmx.Section()
	assert.False(t, sec.Syntax.Header.CurrentNextIndicator)
	assert.Nil(t, dmx.PMT())

	_, err = dmx.Next()
	assert.ErrorIs(t, err, ts.ErrNoMorePackets)
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

// A unit that yields nothing must not displace the cached good unit: the same
// good unit coming back is a repeat, not a change.
func TestDemuxerCacheSurvivesBrokenUnits(t *testing.T) {
	good := patSection(0, true, 0x100)
	broken := patSection(0, true, 0x100)
	broken[len(broken)-1] ^= 0x01

	tests := []struct {
		name     string
		middle   []byte
		wantKind ts.ErrorKind
	}{
		{"pointer_field beyond the unit", payloadPacket(ts.PIDPAT, 1, true, []byte{0xf0}), ts.ErrorKindPSI},
		{"every section damaged", psiPacket(ts.PIDPAT, 1, broken), ts.ErrorKindCRC},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := psiPacket(ts.PIDPAT, 0, good)
			stream = append(stream, tt.middle...)
			stream = append(stream, psiPacket(ts.PIDPAT, 2, good)...)
			dmx := New(context.Background(), bytes.NewReader(stream), WithPacketSize(ts.PacketSize), WithRecoverableErrors())

			var pats, errs int
			for ev, err := range dmx.Events() {
				switch {
				case err != nil:
					var re *ts.RecoverableError
					require.ErrorAs(t, err, &re)
					assert.Equal(t, tt.wantKind, re.Kind)
					errs++
				case ev == EventPAT:
					assert.True(t, dmx.TableChanged())
					pats++
				default:
					t.Fatalf("unexpected event %v", ev)
				}
			}
			assert.Equal(t, 1, pats)
			assert.Equal(t, 1, errs)
		})
	}
}

func TestDemuxerPESPacketOffsets(t *testing.T) {
	t.Run("two packets", func(t *testing.T) {
		stream := payloadPacket(0x100, 0, true, unboundedPES)
		stream = append(stream, payloadPacket(0x100, 1, false, []byte("more"))...)
		stream = append(stream, payloadPacket(0x100, 2, true, unboundedPES)...)
		dmx := New(context.Background(), bytes.NewReader(stream), WithPacketSize(ts.PacketSize))

		ev, err := dmx.Next()
		require.NoError(t, err)
		require.Equal(t, EventPES, ev)
		assert.Equal(t, int64(0), dmx.PES().FirstPacketOffset)
		assert.Equal(t, int64(ts.PacketSize), dmx.PES().LastPacketOffset)
	})

	t.Run("headless prefix", func(t *testing.T) {
		stream := payloadPacket(0x100, 0, false, unboundedPES)
		stream = append(stream, payloadPacket(0x100, 1, true, unboundedPES)...)
		dmx := New(context.Background(), bytes.NewReader(stream), WithPacketSize(ts.PacketSize))

		ev, err := dmx.Next()
		require.NoError(t, err)
		require.Equal(t, EventPES, ev)
		assert.Equal(t, int64(0), dmx.PES().FirstPacketOffset, "the first packet seen starts the headless unit")
		assert.Equal(t, int64(0), dmx.PES().LastPacketOffset)

		ev, err = dmx.Next()
		require.NoError(t, err)
		require.Equal(t, EventPES, ev)
		assert.Equal(t, int64(ts.PacketSize), dmx.PES().FirstPacketOffset)
		assert.Equal(t, int64(ts.PacketSize), dmx.PES().LastPacketOffset)
		assert.True(t, dmx.PES().Truncated)
	})
}
