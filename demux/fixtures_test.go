package demux

import (
	"bytes"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bitstest"
	"github.com/k-danil/go-astits/v2/ts"
)

const syncByte byte = '\x47'

func packet(h ts.PacketHeader, a *ts.PacketAdaptationField, i []byte, packet192bytes bool) ([]byte, *ts.Packet) {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	var prefix []byte
	if packet192bytes {
		prefix = []byte("test") // M2TS TP_extra_header prefix (4 bytes) before the sync byte
		_ = w.Write(prefix)
	}
	_ = w.Write(syncByte)                                           // Sync byte
	_ = w.Write(packetHeaderBytes(h, "11"))                         // Header
	_ = w.Write(packetAdaptationFieldBytes(a))                      // Adaptation field
	var payload = append(i, bytes.Repeat([]byte{0}, 147-len(i))...) // Payload
	_ = w.Write(payload)
	pk := &ts.Packet{
		Header:  packetHeader,
		Payload: payload,
		Prefix:  prefix,
	}
	pk.SetAdaptationField(packetAdaptationField)
	return buf.Bytes(), pk
}

func packetShort(h ts.PacketHeader, payload []byte) ([]byte, *ts.Packet) {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write(syncByte)                   // Sync byte
	_ = w.Write(packetHeaderBytes(h, "01")) // Header
	p := append(payload, bytes.Repeat([]byte{0}, ts.PacketSize-buf.Len())...)
	_ = w.Write(p)
	return buf.Bytes(), &ts.Packet{
		Header:  h,
		Payload: payload,
	}
}

var packetHeader = ts.PacketHeader{
	ContinuityCounter:          10,
	HasAdaptationField:         true,
	HasPayload:                 true,
	PayloadUnitStartIndicator:  true,
	PID:                        5461,
	TransportErrorIndicator:    true,
	TransportPriority:          true,
	TransportScramblingControl: ts.ScramblingControlScrambledWithEvenKey,
}

func packetHeaderBytes(h ts.PacketHeader, afControl string) []byte {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write(h.TransportErrorIndicator)                // Transport error indicator
	_ = w.Write(h.PayloadUnitStartIndicator)              // Payload unit start indicator
	_ = w.Write("1")                                      // Transport priority
	_ = w.Write(fmt.Sprintf("%.13b", h.PID))              // PID
	_ = w.Write("10")                                     // Scrambling control
	_ = w.Write(afControl)                                // Adaptation field control
	_ = w.Write(fmt.Sprintf("%.4b", h.ContinuityCounter)) // Continuity counter
	return buf.Bytes()
}

var packetAdaptationField = &ts.PacketAdaptationField{
	AdaptationExtensionField: &ts.PacketAdaptationExtensionField{
		DTSNextAccessUnit:      dtsClockReference,
		HasLegalTimeWindow:     true,
		HasPiecewiseRate:       true,
		HasSeamlessSplice:      true,
		LegalTimeWindowIsValid: true,
		LegalTimeWindowOffset:  10922,
		Length:                 11,
		PiecewiseRate:          2796202,
		SpliceType:             2,
	},
	DiscontinuityIndicator:            true,
	ElementaryStreamPriorityIndicator: true,
	HasAdaptationExtensionField:       true,
	HasOPCR:                           true,
	HasPCR:                            true,
	HasTransportPrivateData:           true,
	HasSplicingCountdown:              true,
	Length:                            36,
	OPCR:                              pcr,
	PCR:                               pcr,
	RandomAccessIndicator:             true,
	SpliceCountdown:                   2,
	TransportPrivateDataLength:        4,
	TransportPrivateData:              []byte("test"),
	StuffingLength:                    5,
}

func packetAdaptationFieldBytes(a *ts.PacketAdaptationField) []byte {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write(uint8(36))                // Length
	_ = w.Write(a.DiscontinuityIndicator) // Discontinuity indicator
	_ = w.Write("1")                      // Random access indicator
	_ = w.Write("1")                      // Elementary stream priority indicator
	_ = w.Write("1")                      // PCR flag
	_ = w.Write("1")                      // OPCR flag
	_ = w.Write("1")                      // Splicing point flag
	_ = w.Write("1")                      // Transport data flag
	_ = w.Write("1")                      // Adaptation field extension flag
	_ = w.Write(pcrBytes())               // PCR
	_ = w.Write(pcrBytes())               // OPCR
	_ = w.Write(uint8(2))                 // Splice countdown
	_ = w.Write(uint8(4))                 // Transport private data length
	_ = w.Write([]byte("test"))           // Transport private data
	_ = w.Write(uint8(11))                // Adaptation extension length
	_ = w.Write("1")                      // LTW flag
	_ = w.Write("1")                      // Piecewise rate flag
	_ = w.Write("1")                      // Seamless splice flag
	_ = w.Write("11111")                  // Reserved
	_ = w.Write("1")                      // LTW valid flag
	_ = w.Write("010101010101010")        // LTW offset
	_ = w.Write("11")                     // Piecewise rate reserved
	_ = w.Write("1010101010101010101010") // Piecewise rate
	_ = w.Write(dtsBytes("0010"))         // Splice type + DTS next access unit
	_ = w.WriteN(^uint64(0), 40)          // Stuffing bytes
	return buf.Bytes()
}

var pcr = ts.NewClockReference(5726623061, 341)

var dtsClockReference = ts.NewClockReference(5726623060, 0)

func dtsBytes(flag string) []byte {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write(flag)              // Flag
	_ = w.Write("101")             // 32...30
	_ = w.Write("1")               // Dummy
	_ = w.Write("010101010101010") // 29...15
	_ = w.Write("1")               // Dummy
	_ = w.Write("101010101010100") // 14...0
	_ = w.Write("1")               // Dummy
	return buf.Bytes()
}

func pcrBytes() []byte {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write("101010101010101010101010101010101") // Base
	_ = w.Write("111111")                            // Reserved
	_ = w.Write("101010101")                         // Extension
	return buf.Bytes()
}

// Frozen outputs of the former in-package fixture builders (now in psi/pes
// package tests): parse correctness is covered there, root tests only need
// the bytes.
const (
	psiBytesHex           = "04746573744ef01e0001eb02030002000304050006c079124500014530f0035201077ffc610240f0190001eb020300035201070009000200030003520107febaa94100f0110001eb02030002e0030004e00560739f6102f0180001eb0203f555f00352010703eaaaf003520107c68442e842f0140001eb0203000200000303b003520107ef3751d673f00ec079124500000352010706969b13fe00"
	pesWithHeaderBytesHex = "0000010100439fff3c3b5555aaab1b5555aaa9dc2f842e7c75aaaaab35ff0004bf31323334353637383930313233343536d5d575558a657874656e73696f6e327374756666646174617374756666"
)

func psiBytes() []byte { return hexToBytes(psiBytesHex) }

func pesWithHeaderBytes() []byte { return hexToBytes(pesWithHeaderBytesHex) }
