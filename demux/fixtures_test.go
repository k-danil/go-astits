package demux

import (
	"bytes"
	"fmt"

	"github.com/asticode/go-astikit"

	"github.com/k-danil/go-astits/ts"
)

const syncByte byte = '\x47'

func packet(h ts.PacketHeader, a *ts.PacketAdaptationField, i []byte, packet192bytes bool) ([]byte, *ts.Packet) {
	buf := &bytes.Buffer{}
	w := astikit.NewBitsWriter(astikit.BitsWriterOptions{Writer: buf})
	w.Write(syncByte) // Sync byte
	if packet192bytes {
		w.Write([]byte("test")) // Sometimes packets are 192 bytes
	}
	w.Write(packetHeaderBytes(h, "11"))                             // Header
	w.Write(packetAdaptationFieldBytes(a))                          // Adaptation field
	var payload = append(i, bytes.Repeat([]byte{0}, 147-len(i))...) // Payload
	w.Write(payload)
	pk := &ts.Packet{
		Header:  packetHeader,
		Payload: payload,
	}
	pk.SetAdaptationField(packetAdaptationField)
	return buf.Bytes(), pk
}

func packetShort(h ts.PacketHeader, payload []byte) ([]byte, *ts.Packet) {
	buf := &bytes.Buffer{}
	w := astikit.NewBitsWriter(astikit.BitsWriterOptions{Writer: buf})
	w.Write(syncByte)                   // Sync byte
	w.Write(packetHeaderBytes(h, "01")) // Header
	p := append(payload, bytes.Repeat([]byte{0}, ts.MpegTsPacketSize-buf.Len())...)
	w.Write(p)
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
	w := astikit.NewBitsWriter(astikit.BitsWriterOptions{Writer: buf})
	w.Write(h.TransportErrorIndicator)                // Transport error indicator
	w.Write(h.PayloadUnitStartIndicator)              // Payload unit start indicator
	w.Write("1")                                      // Transport priority
	w.Write(fmt.Sprintf("%.13b", h.PID))              // PID
	w.Write("10")                                     // Scrambling control
	w.Write(afControl)                                // Adaptation field control
	w.Write(fmt.Sprintf("%.4b", h.ContinuityCounter)) // Continuity counter
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
	w := astikit.NewBitsWriter(astikit.BitsWriterOptions{Writer: buf})
	w.Write(uint8(36))                // Length
	w.Write(a.DiscontinuityIndicator) // Discontinuity indicator
	w.Write("1")                      // Random access indicator
	w.Write("1")                      // Elementary stream priority indicator
	w.Write("1")                      // PCR flag
	w.Write("1")                      // OPCR flag
	w.Write("1")                      // Splicing point flag
	w.Write("1")                      // Transport data flag
	w.Write("1")                      // Adaptation field extension flag
	w.Write(pcrBytes())               // PCR
	w.Write(pcrBytes())               // OPCR
	w.Write(uint8(2))                 // Splice countdown
	w.Write(uint8(4))                 // Transport private data length
	w.Write([]byte("test"))           // Transport private data
	w.Write(uint8(11))                // Adaptation extension length
	w.Write("1")                      // LTW flag
	w.Write("1")                      // Piecewise rate flag
	w.Write("1")                      // Seamless splice flag
	w.Write("11111")                  // Reserved
	w.Write("1")                      // LTW valid flag
	w.Write("010101010101010")        // LTW offset
	w.Write("11")                     // Piecewise rate reserved
	w.Write("1010101010101010101010") // Piecewise rate
	w.Write(dtsBytes("0010"))         // Splice type + DTS next access unit
	w.WriteN(^uint64(0), 40)          // Stuffing bytes
	return buf.Bytes()
}

var pcr = ts.NewClockReference(5726623061, 341)

var dtsClockReference = ts.NewClockReference(5726623060, 0)

func dtsBytes(flag string) []byte {
	buf := &bytes.Buffer{}
	w := astikit.NewBitsWriter(astikit.BitsWriterOptions{Writer: buf})
	w.Write(flag)              // Flag
	w.Write("101")             // 32...30
	w.Write("1")               // Dummy
	w.Write("010101010101010") // 29...15
	w.Write("1")               // Dummy
	w.Write("101010101010100") // 14...0
	w.Write("1")               // Dummy
	return buf.Bytes()
}

func pcrBytes() []byte {
	buf := &bytes.Buffer{}
	w := astikit.NewBitsWriter(astikit.BitsWriterOptions{Writer: buf})
	w.Write("101010101010101010101010101010101") // Base
	w.Write("111111")                            // Reserved
	w.Write("101010101")                         // Extension
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
