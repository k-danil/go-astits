package demux

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v3/psi"
	"github.com/k-danil/go-astits/v3/ts"
)

const unitPayloadSize = ts.PacketSize - ts.HeaderSize

func unitPackets(pid uint16, cc uint8, unit []byte, dropPacket int) (stream []byte, next uint8) {
	for i := 0; len(unit) > 0; i++ {
		n := min(unitPayloadSize, len(unit))
		if i != dropPacket {
			stream = append(stream, payloadPacket(pid, cc, i == 0, unit[:n])...)
		}
		unit = unit[n:]
		cc = (cc + 1) % 16
	}
	return stream, cc
}

func pesUnit(size int) []byte {
	unit := []byte{0x00, 0x00, 0x01, 0xe0, 0x00, 0x00, 0x80, 0x00, 0x00}
	for i := range size {
		unit = append(unit, byte(i))
	}
	return unit
}

func pmtUnit(streams int) []byte {
	body := append(syntaxHeader(0, true, 0, 0), 0xe2, 0x00, 0xf0, 0x00)
	for i := range streams {
		pid := uint16(0x200 + i)
		body = append(body, 0x1b, 0xe0|byte(pid>>8), byte(pid), 0xf0, 0x00)
	}
	return append([]byte{0x00}, sectionWithCRC(psi.TableIDPMT, body)...)
}

// The remains of a cut unit are counted as headless, never parsed as an unknown payload or as sections.
func TestTornUnitTailIsHeadless(t *testing.T) {
	const pmtPID = 0x100
	for _, tc := range []struct {
		name        string
		pid         uint16
		unit        []byte
		dropPacket  int
		wantEvent   Event
		wantErrs    []error
		wantDropped int64
	}{
		{
			name:        "elementary stream",
			pid:         0x200,
			unit:        pesUnit(4*unitPayloadSize - 9),
			dropPacket:  2,
			wantEvent:   EventPES,
			wantErrs:    []error{ts.ErrContinuityGap, ts.ErrHeadlessUnit},
			wantDropped: 3 * unitPayloadSize,
		},
		{
			name:        "PSI PID",
			pid:         pmtPID,
			unit:        pmtUnit(96),
			dropPacket:  1,
			wantEvent:   EventPMT,
			wantErrs:    []error{ts.ErrContinuityGap, ts.ErrHeadlessUnit},
			wantDropped: 2 * unitPayloadSize,
		},
		{
			name:        "PSI PID joined past the start",
			pid:         pmtPID,
			unit:        pmtUnit(96),
			dropPacket:  0,
			wantEvent:   EventPMT,
			wantErrs:    []error{ts.ErrHeadlessUnit},
			wantDropped: 2 * unitPayloadSize,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stream := psiPacket(ts.PIDPAT, 0, patSection(0, true, pmtPID))
			torn, cc := unitPackets(tc.pid, 0, tc.unit, tc.dropPacket)
			stream = append(stream, torn...)
			whole, _ := unitPackets(tc.pid, cc, tc.unit, -1)
			stream = append(stream, whole...)

			dmx := New(context.Background(), bytes.NewReader(stream), WithPacketSize(ts.PacketSize), WithRecoverableErrors())
			defer dmx.Close()

			var errs []*ts.RecoverableError
			var delivered int
			for ev, err := range dmx.Events() {
				if re, ok := errors.AsType[*ts.RecoverableError](err); ok {
					errs = append(errs, re)
					continue
				}
				require.NoError(t, err)
				if ev == tc.wantEvent {
					delivered++
				}
			}

			require.Len(t, errs, len(tc.wantErrs))
			var dropped int64
			for i, want := range tc.wantErrs {
				assert.Equal(t, ts.ErrorKindTornUnit, errs[i].Kind)
				require.ErrorIs(t, errs[i].Err, want)
				dropped += errs[i].Dropped
			}
			assert.Equal(t, tc.wantDropped, dropped, "every accumulated byte is accounted for")
			assert.Positive(t, delivered, "the unit after the damage still arrives")
		})
	}
}
