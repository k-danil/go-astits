package pes

import (
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v2/ts"
)

const roundtripIterations = 500

func randOptionalHeader(r *rand.Rand) *OptionalHeader {
	h := &OptionalHeader{
		MarkerBits:             2,
		ScramblingControl:      uint8(r.UintN(4)),
		Priority:               r.UintN(2) == 1,
		DataAlignmentIndicator: r.UintN(2) == 1,
		IsCopyrighted:          r.UintN(2) == 1,
		IsOriginal:             r.UintN(2) == 1,
		PTSDTSIndicator:        []uint8{PTSDTSIndicatorNoPTSOrDTS, PTSDTSIndicatorOnlyPTS, PTSDTSIndicatorBothPresent}[r.UintN(3)],
		HasESCR:                r.UintN(2) == 1,
		HasESRate:              r.UintN(2) == 1,
		HasDSMTrickMode:        r.UintN(2) == 1,
		HasAdditionalCopyInfo:  r.UintN(2) == 1,
		HasExtension:           r.UintN(2) == 1,
	}
	if h.PTSDTSIndicator == PTSDTSIndicatorOnlyPTS {
		h.PTS = ts.NewClockReference(uint64(r.Uint64N(1<<33)), 0)
	}
	if h.PTSDTSIndicator == PTSDTSIndicatorBothPresent {
		h.PTS = ts.NewClockReference(uint64(r.Uint64N(1<<33)), 0)
		h.DTS = ts.NewClockReference(uint64(r.Uint64N(1<<33)), 0)
	}
	if h.HasESCR {
		h.ESCR = ts.NewClockReference(uint64(r.Uint64N(1<<33)), uint64(r.UintN(300)))
	}
	if h.HasESRate {
		h.ESRate = uint32(r.UintN(1 << 22))
	}
	if h.HasDSMTrickMode {
		h.DSMTrickMode = &DSMTrickMode{
			TrickModeControl: TrickModeControlSlowMotion,
			RepeatControl:    uint8(r.UintN(32)),
		}
	}
	if h.HasAdditionalCopyInfo {
		h.AdditionalCopyInfo = uint8(r.UintN(128))
	}
	if h.HasExtension {
		e := &OptionalHeaderExtension{
			HasPrivateData:                  r.UintN(2) == 1,
			HasProgramPacketSequenceCounter: r.UintN(2) == 1,
			HasPSTDBuffer:                   r.UintN(2) == 1,
			HasExtension2:                   r.UintN(2) == 1,
		}
		if e.HasPrivateData {
			e.PrivateData = make([]byte, 16)
			for i := range e.PrivateData {
				e.PrivateData[i] = uint8(r.UintN(256))
			}
		}
		if e.HasProgramPacketSequenceCounter {
			e.PacketSequenceCounter = uint8(r.UintN(128))
			e.MPEG1OrMPEG2ID = uint8(r.UintN(2))
			e.OriginalStuffingLength = uint8(r.UintN(64))
		}
		if e.HasPSTDBuffer {
			e.PSTDBufferScale = uint8(r.UintN(2))
			e.PSTDBufferSize = uint16(r.UintN(1 << 13))
		}
		if e.HasExtension2 {
			n := int(r.UintN(16))
			e.Extension2Data = make([]byte, n)
			for i := range e.Extension2Data {
				e.Extension2Data[i] = uint8(r.UintN(256))
			}
			e.Extension2Length = uint8(n)
		}
		h.Extension = e
	}
	return h
}

func TestRoundtripPESData(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	buf := make([]byte, 64<<10)
	for i := 0; i < roundtripIterations; i++ {
		h := Header{
			StreamID:       0xc0, // non-video: packet length gets written
			OptionalHeader: randOptionalHeader(r),
		}
		payload := make([]byte, 1+r.UintN(256))
		for j := range payload {
			payload[j] = uint8(r.UintN(256))
		}

		// Put emits the whole PES including the 00 00 01 prefix
		n, consumed, err := h.Put(buf, payload, true)
		require.NoError(t, err, "iteration %d", i)
		require.Equal(t, len(payload), consumed, "iteration %d", i)
		bs := buf[:n]

		var d Data
		require.NoError(t, d.Parse(bs), "iteration %d", i)
		assert.Equal(t, payload, d.Data, "iteration %d", i)

		bs2 := make([]byte, n)
		n2, consumed2, err := d.Header.Put(bs2, d.Data, true)
		require.NoError(t, err, "iteration %d", i)
		require.Equal(t, consumed, consumed2, "iteration %d", i)
		assert.Equal(t, bs, bs2[:n2], "iteration %d", i)
	}
}
