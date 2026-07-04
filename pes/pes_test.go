package pes

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v2/internal/bitstest"
	"github.com/k-danil/go-astits/v2/ts"
)

func TestHasPESOptionalHeader(t *testing.T) {
	var a []int
	for i := 0; i <= 255; i++ {
		if !hasPESOptionalHeader(uint8(i)) {
			a = append(a, i)
		}
	}
	assert.Equal(t, []int{StreamIDPaddingStream, StreamIDPrivateStream2}, a)
}

var dsmTrickModeSlow = &DSMTrickMode{
	RepeatControl:    21,
	TrickModeControl: TrickModeControlSlowMotion,
}

func dsmTrickModeSlowBytes() []byte {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write("001")   // Control
	_ = w.Write("10101") // Repeat control
	return buf.Bytes()
}

type dsmTrickModeTestCase struct {
	name      string
	bytesFunc func(w *bitstest.Writer)
	trickMode *DSMTrickMode
}

var dsmTrickModeTestCases = []dsmTrickModeTestCase{
	{
		"fast_forward",
		func(w *bitstest.Writer) {
			_ = w.Write("000") // Control
			_ = w.Write("10")  // Field ID
			_ = w.Write("1")   // Intra slice refresh
			_ = w.Write("11")  // Frequency truncation
		},
		&DSMTrickMode{
			FieldID:             2,
			FrequencyTruncation: 3,
			IntraSliceRefresh:   1,
			TrickModeControl:    TrickModeControlFastForward,
		},
	},
	{
		"slow_motion",
		func(w *bitstest.Writer) {
			_ = w.Write("001")
			_ = w.Write("10101")
		},
		&DSMTrickMode{
			RepeatControl:    0b10101,
			TrickModeControl: TrickModeControlSlowMotion,
		},
	},
	{
		"freeze_frame",
		func(w *bitstest.Writer) {
			_ = w.Write("010") // Control
			_ = w.Write("10")  // Field ID
			_ = w.Write("111") // Reserved
		},
		&DSMTrickMode{
			FieldID:          2,
			TrickModeControl: TrickModeControlFreezeFrame,
		},
	},
	{
		"fast_reverse",
		func(w *bitstest.Writer) {
			_ = w.Write("011") // Control
			_ = w.Write("10")  // Field ID
			_ = w.Write("1")   // Intra slice refresh
			_ = w.Write("11")  // Frequency truncation
		},
		&DSMTrickMode{
			FieldID:             2,
			FrequencyTruncation: 3,
			IntraSliceRefresh:   1,
			TrickModeControl:    TrickModeControlFastReverse,
		},
	},
	{
		"slow_reverse",
		func(w *bitstest.Writer) {
			_ = w.Write("100")
			_ = w.Write("01010")
		},
		&DSMTrickMode{
			RepeatControl:    0b01010,
			TrickModeControl: TrickModeControlSlowReverse,
		},
	},
	{
		"reserved",
		func(w *bitstest.Writer) {
			_ = w.Write("101")
			_ = w.Write("11111")
		},
		&DSMTrickMode{
			TrickModeControl: 5, // reserved
		},
	},
}

func TestParseDSMTrickMode(t *testing.T) {
	for _, tc := range dsmTrickModeTestCases {
		t.Run(tc.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			w := bitstest.NewWriter(buf)
			tc.bytesFunc(w)
			assert.Equal(t, parseDSMTrickMode(buf.Bytes()[0]), tc.trickMode)
		})
	}
}

func TestWriteDSMTrickMode(t *testing.T) {
	for _, tc := range dsmTrickModeTestCases {
		t.Run(tc.name, func(t *testing.T) {
			bufExpected := &bytes.Buffer{}
			wExpected := bitstest.NewWriter(bufExpected)
			tc.bytesFunc(wExpected)

			bs := make([]byte, dsmTrickModeLength)
			n := tc.trickMode.putBytes(bs)
			assert.Equal(t, 1, n)
			assert.Equal(t, bufExpected.Bytes(), bs[:n])
		})
	}
}

var ptsClockReference = ts.NewClockReference(5726623061, 0)

func ptsBytes(flag string) []byte {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write(flag)              // Flag
	_ = w.Write("101")             // 32...30
	_ = w.Write("1")               // Dummy
	_ = w.Write("010101010101010") // 29...15
	_ = w.Write("1")               // Dummy
	_ = w.Write("101010101010101") // 14...0
	_ = w.Write("1")               // Dummy
	return buf.Bytes()
}

var dtsClockReference = ts.NewClockReference(5726623060, 0)

var clockReference = ts.NewClockReference(3271034319, 58)

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

func escrBytes() []byte {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write("11")              // Dummy
	_ = w.Write("011")             // 32...30
	_ = w.Write("1")               // Dummy
	_ = w.Write("000010111110000") // 29...15
	_ = w.Write("1")               // Dummy
	_ = w.Write("000010111001111") // 14...0
	_ = w.Write("1")               // Dummy
	_ = w.Write("000111010")       // Ext
	_ = w.Write("1")               // Dummy
	return buf.Bytes()
}

type pesTestCase struct {
	name                    string
	headerBytesFunc         func(w *bitstest.Writer, withStuffing bool, withCRC bool)
	optionalHeaderBytesFunc func(w *bitstest.Writer, withStuffing bool, withCRC bool)
	bytesFunc               func(w *bitstest.Writer, withStuffing bool, withCRC bool)
	pesData                 *Data
}

var pesTestCases = []pesTestCase{
	{
		"without_header",
		func(w *bitstest.Writer, withStuffing bool, withCRC bool) {
			_ = w.Write("000000000000000000000001")   // Prefix
			_ = w.Write(uint8(StreamIDPaddingStream)) // Stream ID
			_ = w.Write(uint16(4))                    // Packet length
		},
		func(w *bitstest.Writer, withStuffing bool, withCRC bool) {
			// do nothing here
		},
		func(w *bitstest.Writer, withStuffing bool, withCRC bool) {
			_ = w.Write([]byte("data")) // Data
		},
		&Data{
			Data: []byte("data"),
			Header: Header{
				PacketLength: 4,
				StreamID:     StreamIDPaddingStream,
			},
		},
	},
	{
		"with_header",
		func(w *bitstest.Writer, withStuffing bool, withCRC bool) {
			packetLength := 67
			stuffing := []byte("stuff")

			if !withStuffing {
				packetLength -= len(stuffing)
			}

			if !withCRC {
				packetLength -= 2
			}

			_ = w.Write("000000000000000000000001") // Prefix
			_ = w.Write(uint8(1))                   // Stream ID
			_ = w.Write(uint16(packetLength))       // Packet length

		},
		func(w *bitstest.Writer, withStuffing bool, withCRC bool) {
			optionalHeaderLength := 60
			stuffing := []byte("stuff")

			if !withStuffing {
				optionalHeaderLength -= len(stuffing)
			}

			if !withCRC {
				optionalHeaderLength -= 2
			}

			_ = w.Write("10")                        // Marker bits
			_ = w.Write("01")                        // Scrambling control
			_ = w.Write("1")                         // Priority
			_ = w.Write("1")                         // Data alignment indicator
			_ = w.Write("1")                         // Copyright
			_ = w.Write("1")                         // Original or copy
			_ = w.Write("11")                        // PTS/DTS indicator
			_ = w.Write("1")                         // ESCR flag
			_ = w.Write("1")                         // ES rate flag
			_ = w.Write("1")                         // DSM trick mode flag
			_ = w.Write("1")                         // Additional copy flag
			_ = w.Write(withCRC)                     // CRC flag
			_ = w.Write("1")                         // Extension flag
			_ = w.Write(uint8(optionalHeaderLength)) // Header length
			_ = w.Write(ptsBytes("0011"))            // PTS
			_ = w.Write(dtsBytes("0001"))            // DTS
			_ = w.Write(escrBytes())                 // ESCR
			_ = w.Write("101010101010101010101011")  // ES rate
			_ = w.Write(dsmTrickModeSlowBytes())     // DSM trick mode
			_ = w.Write("11111111")                  // Additional copy info
			if withCRC {
				_ = w.Write(uint16(4)) // CRC
			}
			// Extension starts here
			_ = w.Write("1")                        // Private data flag
			_ = w.Write("0")                        // Pack header field flag
			_ = w.Write("1")                        // Program packet sequence counter flag
			_ = w.Write("1")                        // PSTD buffer flag
			_ = w.Write("111")                      // Dummy
			_ = w.Write("1")                        // Extension 2 flag
			_ = w.Write([]byte("1234567890123456")) // Private data
			//w.Write(uint8(5))                   // Pack field
			_ = w.Write("1101010111010101")   // Packet sequence counter
			_ = w.Write("0111010101010101")   // PSTD buffer
			_ = w.Write("10001010")           // Extension 2 header
			_ = w.Write([]byte("extension2")) // Extension 2 data
			if withStuffing {
				_ = w.Write(stuffing) // Optional header stuffing bytes
			}
		},
		func(w *bitstest.Writer, withStuffing bool, withCRC bool) {
			stuffing := []byte("stuff")
			_ = w.Write([]byte("data")) // Data
			if withStuffing {
				_ = w.Write(stuffing) // Stuffing
			}
		},
		&Data{
			Data: []byte("data"),
			Header: Header{
				OptionalHeader: &OptionalHeader{
					AdditionalCopyInfo:     127,
					CRC:                    4,
					DataAlignmentIndicator: true,
					DSMTrickMode:           dsmTrickModeSlow,
					DTS:                    dtsClockReference,
					ESCR:                   clockReference,
					ESRate:                 1398101,
					HasAdditionalCopyInfo:  true,
					HasCRC:                 true,
					HasDSMTrickMode:        true,
					HasESCR:                true,
					HasESRate:              true,
					HasExtension:           true,
					HeaderLength:           60,
					IsCopyrighted:          true,
					IsOriginal:             true,
					MarkerBits:             2,
					//PackField:                       5,
					Priority:          true,
					PTSDTSIndicator:   3,
					PTS:               ptsClockReference,
					ScramblingControl: 1,
					Extension: &OptionalHeaderExtension{
						PrivateData:                     []byte("1234567890123456"),
						PSTDBufferScale:                 1,
						PSTDBufferSize:                  5461,
						MPEG1OrMPEG2ID:                  1,
						OriginalStuffingLength:          21,
						PacketSequenceCounter:           85,
						HasExtension2:                   true,
						HasPackHeaderField:              false,
						HasPrivateData:                  true,
						HasProgramPacketSequenceCounter: true,
						HasPSTDBuffer:                   true,
						Extension2Data:                  []byte("extension2"),
						Extension2Length:                10,
					},
				},
				PacketLength: 67,
				StreamID:     1,
			},
		},
	},
}

// used by TestParseData
func pesWithHeaderBytes() []byte {
	buf := bytes.Buffer{}
	w := bitstest.NewWriter(&buf)
	pesTestCases[1].headerBytesFunc(w, true, true)
	pesTestCases[1].optionalHeaderBytesFunc(w, true, true)
	pesTestCases[1].bytesFunc(w, true, true)
	return buf.Bytes()
}

// used by TestParseData
func pesWithHeader() *Data {
	return pesTestCases[1].pesData
}

// embedPESFixture normalizes a fixture to its post-parse shape: OptionalHeader
// points into the embedded Header storage
func embedPESFixture(pd *Data) *Data {
	if pd.Header.OptionalHeader != nil && pd.Header.OptionalHeader != &pd.Header.optionalHeader {
		pd.Header.optionalHeader = *pd.Header.OptionalHeader
		pd.Header.OptionalHeader = &pd.Header.optionalHeader
	}
	return pd
}

func TestParsePESData(t *testing.T) {
	for _, tc := range pesTestCases {
		t.Run(tc.name, func(t *testing.T) {
			buf := bytes.Buffer{}
			w := bitstest.NewWriter(&buf)
			tc.headerBytesFunc(w, true, true)
			tc.optionalHeaderBytesFunc(w, true, true)
			tc.bytesFunc(w, true, true)
			d := &Data{}
			err := d.Parse(buf.Bytes())
			assert.NoError(t, err)
			assert.Equal(t, embedPESFixture(tc.pesData), d)
		})
	}
}

// writableHeader clears the flags whose write path is guarded by
// ErrUnsupportedHeaderWrite, keeping the rest of the fixture intact.
func writableHeader(h Header) Header {
	if h.OptionalHeader != nil {
		oh := *h.OptionalHeader
		oh.HasCRC = false
		h.OptionalHeader = &oh
	}
	return h
}

func TestWritePESData(t *testing.T) {
	for _, tc := range pesTestCases {
		t.Run(tc.name, func(t *testing.T) {
			bufExpected := bytes.Buffer{}
			wExpected := bitstest.NewWriter(&bufExpected)
			tc.headerBytesFunc(wExpected, false, false)
			tc.optionalHeaderBytesFunc(wExpected, false, false)
			tc.bytesFunc(wExpected, false, false)

			var bufActual bytes.Buffer
			scratch := make([]byte, ts.PacketSize-ts.HeaderSize)

			start := true
			totalBytes := 0
			payloadPos := 0

			wh := writableHeader(tc.pesData.Header)
			for payloadPos+1 < len(tc.pesData.Data) {
				n, payloadN, err := wh.Put(
					scratch,
					tc.pesData.Data[payloadPos:],
					start,
				)
				assert.NoError(t, err)
				start = false

				bufActual.Write(scratch[:n])
				totalBytes += n
				payloadPos += payloadN
			}

			assert.Equal(t, totalBytes, bufActual.Len())
			assert.Equal(t, bufExpected.Len(), bufActual.Len())
			assert.Equal(t, bufExpected.Bytes(), bufActual.Bytes())
		})
	}
}

func TestWritePESHeader(t *testing.T) {
	for _, tc := range pesTestCases {
		t.Run(tc.name, func(t *testing.T) {
			bufExpected := bytes.Buffer{}
			wExpected := bitstest.NewWriter(&bufExpected)
			tc.headerBytesFunc(wExpected, false, false)
			tc.optionalHeaderBytesFunc(wExpected, false, false)

			bs := make([]byte, ts.PacketSize)
			wh := writableHeader(tc.pesData.Header)
			n, err := wh.putBytes(bs, len(tc.pesData.Data))
			assert.NoError(t, err)
			assert.Equal(t, bufExpected.Len(), n)
			assert.Equal(t, bufExpected.Bytes(), bs[:n])
		})
	}
}

func BenchmarkWritePESHeader(b *testing.B) {
	bs := make([]byte, ts.PacketSize)

	for _, tc := range pesTestCases {
		wh := writableHeader(tc.pesData.Header)
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				wh.putBytes(bs, len(tc.pesData.Data))
			}
		})
	}
}

func TestWritePESOptionalHeader(t *testing.T) {
	for _, tc := range pesTestCases {
		t.Run(tc.name, func(t *testing.T) {
			bufExpected := bytes.Buffer{}
			wExpected := bitstest.NewWriter(&bufExpected)
			tc.optionalHeaderBytesFunc(wExpected, false, false)

			bs := make([]byte, ts.PacketSize)
			n := tc.pesData.Header.OptionalHeader.putBytes(bs)
			assert.Equal(t, bufExpected.Len(), n)
			assert.True(t, bytes.Equal(bufExpected.Bytes(), bs[:n]))
		})
	}
}

func BenchmarkParsePESData(b *testing.B) {
	bss := make([][]byte, len(pesTestCases))

	for ti, tc := range pesTestCases {
		buf := bytes.Buffer{}
		w := bitstest.NewWriter(&buf)
		tc.headerBytesFunc(w, true, true)
		tc.optionalHeaderBytesFunc(w, true, true)
		tc.bytesFunc(w, true, true)
		bss[ti] = buf.Bytes()
	}

	d := &Data{}
	for ti, tc := range pesTestCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				d.Parse(bss[ti])
				*d = Data{}
			}
		})
	}
}

// The pack_header body is length-prefixed and unmodeled: the parser must skip
// it so the fields after it stay aligned.
func TestParseOptionalHeaderSkipsPackHeader(t *testing.T) {
	buf := bytes.Buffer{}
	w := bitstest.NewWriter(&buf)
	w.Write("10")               // Marker bits
	w.Write("00")               // Scrambling control
	w.Write("0")                // Priority
	w.Write("0")                // Data alignment indicator
	w.Write("0")                // Copyright
	w.Write("0")                // Original or copy
	w.Write("00")               // PTS/DTS indicator
	w.Write("0")                // ESCR flag
	w.Write("0")                // ES rate flag
	w.Write("0")                // DSM trick mode flag
	w.Write("0")                // Additional copy flag
	w.Write("0")                // CRC flag
	w.Write("1")                // Extension flag
	w.Write(uint8(9))           // Header length
	w.Write("0")                // Private data flag
	w.Write("1")                // Pack header field flag
	w.Write("0")                // Program packet sequence counter flag
	w.Write("1")                // PSTD buffer flag
	w.Write("111")              // Dummy
	w.Write("0")                // Extension 2 flag
	w.Write(uint8(4))           // Pack field: pack_header length
	w.Write(uint32(0xdeadbeef)) // pack_header body to be skipped
	w.Write("0111010101010101") // PSTD buffer

	var h OptionalHeader
	_, err := h.parseBytes(buf.Bytes(), 0)
	require.NoError(t, err)
	assert.True(t, h.HasExtension)
	assert.Equal(t, uint8(4), h.Extension.PackField)
	assert.True(t, h.Extension.HasPSTDBuffer)
	assert.Equal(t, uint8(1), h.Extension.PSTDBufferScale)
	assert.Equal(t, uint16(0x1555), h.Extension.PSTDBufferSize)
}

func TestPutUnsupportedHeader(t *testing.T) {
	h := Header{
		StreamID:       0xc0,
		OptionalHeader: &OptionalHeader{MarkerBits: 2, HasCRC: true},
	}
	_, _, err := h.Put(make([]byte, 256), []byte("data"), true)
	assert.ErrorIs(t, err, ErrUnsupportedHeaderWrite)

	h.OptionalHeader = &OptionalHeader{MarkerBits: 2, Extension: &OptionalHeaderExtension{HasPackHeaderField: true}, HasExtension: true}
	_, _, err = h.Put(make([]byte, 256), []byte("data"), true)
	assert.ErrorIs(t, err, ErrUnsupportedHeaderWrite)
}
