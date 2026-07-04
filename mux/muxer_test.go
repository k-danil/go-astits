package mux

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v2/demux"
	"github.com/k-danil/go-astits/v2/descriptor"
	"github.com/k-danil/go-astits/v2/internal/bitstest"
	"github.com/k-danil/go-astits/v2/pes"
	"github.com/k-danil/go-astits/v2/psi"
	"github.com/k-danil/go-astits/v2/ts"
)

const syncByte byte = '\x47'

func patExpectedBytes(versionNumber uint8, cc uint8) []byte {
	buf := bytes.Buffer{}
	w := bitstest.NewWriter(&buf)
	_ = w.Write(syncByte)
	_ = w.Write("010") // no transport error, payload start, no priority
	_ = w.WriteN(ts.PIDPAT, 13)
	_ = w.Write("0001") // no scrambling, no AF, payload present
	_ = w.WriteN(cc, 4)

	_ = w.Write(uint16(0))       // Table ID
	_ = w.Write("1011")          // Syntax section indicator, private bit, reserved
	_ = w.WriteN(uint16(13), 12) // Section length

	_ = w.Write(uint16(psi.TableIDPAT))
	_ = w.Write("11")              // Reserved bits
	_ = w.WriteN(versionNumber, 5) // Version number
	_ = w.Write("1")               // Current/next indicator
	_ = w.Write(uint8(0))          // Section number
	_ = w.Write(uint8(0))          // Last section number

	_ = w.Write(programNumberStart)
	_ = w.Write("111") // reserved
	_ = w.WriteN(pmtStartPID, 13)

	// CRC32
	if versionNumber == 0 {
		_ = w.Write([]byte{0x71, 0x10, 0xd8, 0x78})
	} else {
		_ = w.Write([]byte{0xef, 0xbe, 0x08, 0x5a})
	}

	_ = w.Write(bytes.Repeat([]byte{0xff}, 167))

	return buf.Bytes()
}

func TestMuxer_generatePAT(t *testing.T) {
	muxer := New(context.Background(), nil)

	err := muxer.generatePAT()
	assert.NoError(t, err)
	assert.Equal(t, ts.PacketSize, muxer.patBytes.Len())
	assert.Equal(t, patExpectedBytes(0, 0), muxer.patBytes.Bytes())

	// Version number shouldn't change
	err = muxer.generatePAT()
	assert.NoError(t, err)
	assert.Equal(t, ts.PacketSize, muxer.patBytes.Len())
	assert.Equal(t, patExpectedBytes(0, 1), muxer.patBytes.Bytes())

	// Version number should change
	muxer.pmUpdated = true
	err = muxer.generatePAT()
	assert.NoError(t, err)
	assert.Equal(t, ts.PacketSize, muxer.patBytes.Len())
	assert.Equal(t, patExpectedBytes(1, 2), muxer.patBytes.Bytes())
}

func pmtExpectedBytesVideoOnly(versionNumber, cc uint8) []byte {
	buf := bytes.Buffer{}
	w := bitstest.NewWriter(&buf)
	_ = w.Write(syncByte)
	_ = w.Write("010") // no transport error, payload start, no priority
	_ = w.WriteN(pmtStartPID, 13)
	_ = w.Write("0001") // no scrambling, no AF, payload present
	_ = w.WriteN(cc, 4)

	_ = w.Write(uint16(psi.TableIDPMT)) // Table ID
	_ = w.Write("1011")                 // Syntax section indicator, private bit, reserved
	_ = w.WriteN(uint16(18), 12)        // Section length

	_ = w.Write(programNumberStart)
	_ = w.Write("11")              // Reserved bits
	_ = w.WriteN(versionNumber, 5) // Version number
	_ = w.Write("1")               // Current/next indicator
	_ = w.Write(uint8(0))          // Section number
	_ = w.Write(uint8(0))          // Last section number

	_ = w.Write("111")               // reserved
	_ = w.WriteN(uint16(0x1234), 13) // PCR PID

	_ = w.Write("1111")         // reserved
	_ = w.WriteN(uint16(0), 12) // program info length

	_ = w.Write(uint8(psi.StreamTypeH264Video))
	_ = w.Write("111") // reserved
	_ = w.WriteN(uint16(0x1234), 13)

	_ = w.Write("1111")         // reserved
	_ = w.WriteN(uint16(0), 12) // es info length

	_ = w.Write([]byte{0x31, 0x48, 0x5b, 0xa2}) // CRC32

	_ = w.Write(bytes.Repeat([]byte{0xff}, 162))

	return buf.Bytes()
}

func pmtExpectedBytesVideoAndAudio(versionNumber uint8, cc uint8) []byte {
	buf := bytes.Buffer{}
	w := bitstest.NewWriter(&buf)
	_ = w.Write(syncByte)
	_ = w.Write("010") // no transport error, payload start, no priority
	_ = w.WriteN(pmtStartPID, 13)
	_ = w.Write("0001") // no scrambling, no AF, payload present
	_ = w.WriteN(cc, 4)

	_ = w.Write(uint16(psi.TableIDPMT)) // Table ID
	_ = w.Write("1011")                 // Syntax section indicator, private bit, reserved
	_ = w.WriteN(uint16(23), 12)        // Section length

	_ = w.Write(programNumberStart)
	_ = w.Write("11")              // Reserved bits
	_ = w.WriteN(versionNumber, 5) // Version number
	_ = w.Write("1")               // Current/next indicator
	_ = w.Write(uint8(0))          // Section number
	_ = w.Write(uint8(0))          // Last section number

	_ = w.Write("111")               // reserved
	_ = w.WriteN(uint16(0x1234), 13) // PCR PID

	_ = w.Write("1111")         // reserved
	_ = w.WriteN(uint16(0), 12) // program info length

	_ = w.Write(uint8(psi.StreamTypeH264Video))
	_ = w.Write("111") // reserved
	_ = w.WriteN(uint16(0x1234), 13)
	_ = w.Write("1111")         // reserved
	_ = w.WriteN(uint16(0), 12) // es info length

	_ = w.Write(uint8(psi.StreamTypeADTS))
	_ = w.Write("111") // reserved
	_ = w.WriteN(uint16(0x0234), 13)
	_ = w.Write("1111")         // reserved
	_ = w.WriteN(uint16(0), 12) // es info length

	// CRC32
	if versionNumber == 0 {
		_ = w.Write([]byte{0x29, 0x52, 0xc4, 0x50})
	} else {
		_ = w.Write([]byte{0x06, 0xf4, 0xa6, 0xea})
	}

	_ = w.Write(bytes.Repeat([]byte{0xff}, 157))

	return buf.Bytes()
}

func TestMuxer_generatePMT(t *testing.T) {
	muxer := New(context.Background(), nil)
	err := muxer.AddElementaryStream(psi.ElementaryStream{
		ElementaryPID: 0x1234,
		StreamType:    psi.StreamTypeH264Video,
	})
	muxer.SetPCRPID(0x1234)
	assert.NoError(t, err)

	err = muxer.generatePMT()
	assert.NoError(t, err)
	assert.Equal(t, ts.PacketSize, muxer.pmtBytes.Len())
	assert.Equal(t, pmtExpectedBytesVideoOnly(0, 0), muxer.pmtBytes.Bytes())

	// Version number shouldn't change
	err = muxer.generatePMT()
	assert.NoError(t, err)
	assert.Equal(t, ts.PacketSize, muxer.pmtBytes.Len())
	assert.Equal(t, pmtExpectedBytesVideoOnly(0, 1), muxer.pmtBytes.Bytes())

	err = muxer.AddElementaryStream(psi.ElementaryStream{
		ElementaryPID: 0x0234,
		StreamType:    psi.StreamTypeAACAudio,
	})
	assert.NoError(t, err)

	// Version number should change
	err = muxer.generatePMT()
	assert.NoError(t, err)
	assert.Equal(t, ts.PacketSize, muxer.pmtBytes.Len())
	assert.Equal(t, pmtExpectedBytesVideoAndAudio(1, 2), muxer.pmtBytes.Bytes())
}

func TestMuxer_WriteTables(t *testing.T) {
	buf := bytes.Buffer{}
	muxer := New(context.Background(), &buf)
	err := muxer.AddElementaryStream(psi.ElementaryStream{
		ElementaryPID: 0x1234,
		StreamType:    psi.StreamTypeH264Video,
	})
	muxer.SetPCRPID(0x1234)
	assert.NoError(t, err)

	n, err := muxer.WriteTables()
	assert.NoError(t, err)
	assert.Equal(t, 2*ts.PacketSize, n)
	assert.Equal(t, n, buf.Len())

	expectedBytes := append(patExpectedBytes(0, 0), pmtExpectedBytesVideoOnly(0, 0)...)
	assert.Equal(t, expectedBytes, buf.Bytes())
}

func TestMuxer_WriteTables_Error(t *testing.T) {
	muxer := New(context.Background(), nil)
	err := muxer.AddElementaryStream(psi.ElementaryStream{
		ElementaryPID: 0x1234,
		StreamType:    psi.StreamTypeH264Video,
	})
	assert.NoError(t, err)

	_, err = muxer.WriteTables()
	assert.Equal(t, ErrPCRPIDInvalid, err)
}

func TestMuxer_AddElementaryStream(t *testing.T) {
	muxer := New(context.Background(), nil)
	err := muxer.AddElementaryStream(psi.ElementaryStream{
		ElementaryPID: 0x1234,
		StreamType:    psi.StreamTypeH264Video,
	})
	assert.NoError(t, err)

	err = muxer.AddElementaryStream(psi.ElementaryStream{
		ElementaryPID: 0x1234,
		StreamType:    psi.StreamTypeH264Video,
	})
	assert.Equal(t, ErrPIDAlreadyExists, err)
}

func TestMuxer_RemoveElementaryStream(t *testing.T) {
	muxer := New(context.Background(), nil)
	err := muxer.AddElementaryStream(psi.ElementaryStream{
		ElementaryPID: 0x1234,
		StreamType:    psi.StreamTypeH264Video,
	})
	assert.NoError(t, err)

	err = muxer.RemoveElementaryStream(0x1234)
	assert.NoError(t, err)

	err = muxer.RemoveElementaryStream(0x1234)
	assert.Equal(t, ErrPIDNotFound, err)
}

func testPayload() []byte {
	ret := make([]byte, 0xff+1)
	for i := 0; i <= 0xff; i++ {
		ret[i] = byte(i)
	}
	return ret
}

func TestMuxer_WritePayload(t *testing.T) {
	buf := bytes.Buffer{}
	muxer := New(context.Background(), &buf)

	err := muxer.AddElementaryStream(psi.ElementaryStream{
		ElementaryPID: 0x1234,
		StreamType:    psi.StreamTypeH264Video,
	})
	muxer.SetPCRPID(0x1234)
	assert.NoError(t, err)

	err = muxer.AddElementaryStream(psi.ElementaryStream{
		ElementaryPID: 0x0234,
		StreamType:    psi.StreamTypeAACAudio,
	})
	assert.NoError(t, err)

	payload := testPayload()
	pcr := ts.NewClockReference(5726623061, 341)
	pts := ts.NewClockReference(5726623060, 0)

	n, err := muxer.WriteData(&Data{
		PID: 0x1234,
		AdaptationField: &ts.PacketAdaptationField{
			HasPCR:                true,
			PCR:                   pcr,
			RandomAccessIndicator: true,
		},
		PES: &pes.Data{
			Data: payload,
			Header: pes.Header{
				OptionalHeader: &pes.OptionalHeader{
					DTS:             pts,
					PTS:             pts,
					PTSDTSIndicator: pes.PTSDTSIndicatorBothPresent,
				},
			},
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, buf.Len(), n)

	bytesTotal := n

	n, err = muxer.WriteData(&Data{
		PID: 0x0234,
		AdaptationField: &ts.PacketAdaptationField{
			HasPCR:                true,
			PCR:                   pcr,
			RandomAccessIndicator: true,
		},
		PES: &pes.Data{
			Data: payload,
			Header: pes.Header{
				OptionalHeader: &pes.OptionalHeader{
					DTS:             pts,
					PTS:             pts,
					PTSDTSIndicator: pes.PTSDTSIndicatorBothPresent,
				},
			},
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, buf.Len(), bytesTotal+n)
	assert.Equal(t, 0, buf.Len()%ts.PacketSize)

	bs := buf.Bytes()
	assert.Equal(t, patExpectedBytes(0, 0), bs[:ts.PacketSize])
	assert.Equal(t, pmtExpectedBytesVideoAndAudio(0, 0), bs[ts.PacketSize:ts.PacketSize*2])
}

// A PAT with more programs than fit one section must span sections and
// packets, and survive a full mux->demux cycle.
func TestWriteTablesMultiSectionPAT(t *testing.T) {
	buf := &bytes.Buffer{}
	m := New(context.Background(), buf)
	require.NoError(t, m.AddElementaryStream(psi.ElementaryStream{
		ElementaryPID: 0x123,
		StreamType:    psi.StreamTypeH264Video,
	}))
	m.SetPCRPID(0x123)

	const extraPrograms = 300
	for i := 0; i < extraPrograms; i++ {
		m.pm.Set(uint16(0x200+i), uint16(i+1))
	}
	m.pmUpdated = true

	_, err := m.WriteTables()
	require.NoError(t, err)

	dmx := demux.New(context.Background(), bytes.NewReader(buf.Bytes()), demux.WithPacketSize(ts.PacketSize))
	got := map[uint16]uint16{}
	for {
		ev, derr := dmx.Next()
		if errors.Is(derr, ts.ErrNoMorePackets) {
			break
		}
		require.NoError(t, derr)
		if ev != demux.EventPAT {
			continue
		}
		if _, data := dmx.Section(); data != nil {
			if pat, isPAT := data.(*psi.PAT); isPAT {
				for _, p := range pat.Programs {
					got[p.ProgramMapID] = p.ProgramNumber
				}
			}
		}
	}
	assert.Len(t, got, extraPrograms+1)
	assert.Equal(t, uint16(42+1), got[uint16(0x200+42)])
}

func TestWriteTablesSectionOverflow(t *testing.T) {
	m := New(context.Background(), &bytes.Buffer{})
	for i := 0; i < 5; i++ {
		require.NoError(t, m.AddElementaryStream(psi.ElementaryStream{
			ElementaryPID: uint16(0x100 + i),
			StreamType:    psi.StreamTypeH264Video,
			ElementaryStreamDescriptors: []descriptor.Descriptor{
				&descriptor.UserDefined{
					Header: descriptor.Header{Tag: descriptor.Tag(0x80)},
					Data:   bytes.Repeat([]byte{0xab}, 220),
				},
			},
		}))
	}
	m.SetPCRPID(0x100)

	_, err := m.WriteTables()
	assert.ErrorIs(t, err, psi.ErrSectionOverflow)
}
