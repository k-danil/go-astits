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

const (
	packetPayloadBytes  = ts.PacketSize - ts.HeaderSize
	sectionStartPayload = packetPayloadBytes - 1 // pointer_field takes the first byte
)

// §2.4.4.2: the packet a section starts in carries the start indicator and a
// pointer_field to it, behind whatever the previous section left over.
func framePSI(sections [][]byte) (stream []byte) {
	var payload []byte
	starts := make([]int, 0, len(sections))
	for _, s := range sections {
		starts = append(starts, len(payload))
		payload = append(payload, s...)
	}

	cc := uint8(0)
	for pos := 0; pos < len(payload); {
		next := -1
		for _, st := range starts {
			if st >= pos {
				next = st
				break
			}
		}
		room, pf, pusi := packetPayloadBytes, 0, false
		switch {
		case next < 0:
		case next-pos < sectionStartPayload:
			room, pf, pusi = sectionStartPayload, next-pos, true
		default:
			room = min(room, next-pos)
		}

		var pay []byte
		if pusi {
			pay = append(pay, byte(pf))
		}
		end := min(pos+room, len(payload))
		stream = append(stream, payloadPacket(ts.PIDPAT, cc, pusi, append(pay, payload[pos:end]...))...)
		cc = (cc + 1) % 16
		pos = end
	}
	return
}

func bigPATSection(section, last uint8, programs int) []byte {
	body := syntaxHeader(0, true, section, last)
	for i := range programs {
		pid := 0x100 + uint16(i)
		body = append(body, byte((i+1)>>8), byte(i+1), 0xe0|byte(pid>>8), byte(pid))
	}
	return sectionWithCRC(psi.TableIDPAT, body)
}

// A metadata_section: its body is free-form, so the section can be sized to fill a packet payload to the byte.
func exactFitSection() []byte {
	const crcBytes = 4
	body := bytes.Repeat([]byte{0x5a}, sectionStartPayload-psiSectionHeaderLen-crcBytes)
	l := len(body) + crcBytes
	s := append([]byte{byte(psi.TableIDMetadata), 0x80 | byte(l>>8), byte(l)}, body...)
	return binary.BigEndian.AppendUint32(s, ts.ComputeCRC32(s))
}

// Dropping the bytes ahead of pointer_field loses the section the previous packet left open.
func TestSectionTailCarriedAcrossUnitStart(t *testing.T) {
	tests := []struct {
		name         string
		stream       []byte
		wantSections []uint8
		wantLast     []int64
	}{
		{
			name:         "a section spanning two starts",
			stream:       framePSI([][]byte{bigPATSection(0, 1, 50), patSectionAt(0, true, 1, 1, 999, 0x300)}),
			wantSections: []uint8{0, 1},
			wantLast:     []int64{ts.PacketSize, ts.PacketSize},
		},
		{
			// The unit closed on the packet boundary, so the next pointer_field points at nothing to carry.
			name: "a section ending on the packet boundary carries nothing",
			stream: append(
				payloadPacket(ts.PIDPAT, 0, true, append([]byte{0x00}, exactFitSection()...)),
				payloadPacket(ts.PIDPAT, 1, true, append([]byte{0x05, 'j', 'u', 'n', 'k', '!'}, patSection(0, true, 0x300)...))...,
			),
			wantSections: []uint8{0},
			wantLast:     []int64{ts.PacketSize},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dmx := New(context.Background(), bytes.NewReader(tt.stream), WithPacketSize(ts.PacketSize), WithRecoverableErrors())
			defer dmx.Close()

			var got []uint8
			var last []int64
			for ev, err := range dmx.Events() {
				require.NoError(t, err)
				require.Equal(t, EventPAT, ev)
				_, sec := dmx.Section()
				got = append(got, sec.Syntax.Header.SectionNumber)
				last = append(last, dmx.SectionSpan().LastPacketOffset)
			}
			assert.Equal(t, tt.wantSections, got)
			assert.Equal(t, tt.wantLast, last, "a carried tail ends the section on the packet that carried it")
		})
	}
}
