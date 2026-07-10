package ts

import (
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const roundtripIterations = 500

func roundtripRand() *rand.Rand {
	return rand.New(rand.NewPCG(1, 2))
}

func randBytes(r *rand.Rand, n int) []byte {
	bs := make([]byte, n)
	for i := range bs {
		bs[i] = uint8(r.UintN(256))
	}
	return bs
}

func TestRoundtripPacketHeader(t *testing.T) {
	r := roundtripRand()
	for i := 0; i < roundtripIterations; i++ {
		h := PacketHeader{
			ContinuityCounter:          uint8(r.UintN(16)),
			HasAdaptationField:         r.UintN(2) == 1,
			HasPayload:                 r.UintN(2) == 1,
			PayloadUnitStartIndicator:  r.UintN(2) == 1,
			PID:                        uint16(r.UintN(1 << 13)),
			TransportErrorIndicator:    r.UintN(2) == 1,
			TransportPriority:          r.UintN(2) == 1,
			TransportScramblingControl: uint8(r.UintN(4)),
		}
		var b1, b2 [HeaderSize]byte
		h.Put(b1[:])

		var parsed PacketHeader
		_, err := parsed.Parse(b1[:])
		require.NoError(t, err)
		parsed.Put(b2[:])
		assert.Equal(t, b1, b2, "iteration %d", i)
		assert.Equal(t, h, parsed, "iteration %d", i)
	}
}

func randAdaptationField(r *rand.Rand) *PacketAdaptationField {
	af := &PacketAdaptationField{
		DiscontinuityIndicator:            r.UintN(2) == 1,
		RandomAccessIndicator:             r.UintN(2) == 1,
		ElementaryStreamPriorityIndicator: r.UintN(2) == 1,
		HasPCR:                            r.UintN(2) == 1,
		HasOPCR:                           r.UintN(2) == 1,
		HasSplicingCountdown:              r.UintN(2) == 1,
		HasTransportPrivateData:           r.UintN(2) == 1,
		HasAdaptationExtensionField:       r.UintN(2) == 1,
		StuffingLength:                    uint8(r.UintN(8)),
	}
	if af.HasPCR {
		af.PCR = NewClockReference(uint64(r.Uint64N(1<<33)), uint64(r.UintN(300)))
	}
	if af.HasOPCR {
		af.OPCR = NewClockReference(uint64(r.Uint64N(1<<33)), uint64(r.UintN(300)))
	}
	if af.HasSplicingCountdown {
		af.SpliceCountdown = int8(r.UintN(256))
	}
	if af.HasTransportPrivateData {
		af.TransportPrivateDataLength = uint8(r.UintN(32))
		af.TransportPrivateData = randBytes(r, int(af.TransportPrivateDataLength))
	}
	if af.HasAdaptationExtensionField {
		afe := &PacketAdaptationExtensionField{
			HasLegalTimeWindow: r.UintN(2) == 1,
			HasPiecewiseRate:   r.UintN(2) == 1,
			HasSeamlessSplice:  r.UintN(2) == 1,
		}
		if afe.HasLegalTimeWindow {
			afe.LegalTimeWindowIsValid = r.UintN(2) == 1
			afe.LegalTimeWindowOffset = uint16(r.UintN(1 << 15))
		}
		if afe.HasPiecewiseRate {
			afe.PiecewiseRate = uint32(r.UintN(1 << 22))
		}
		if afe.HasSeamlessSplice {
			afe.SpliceType = uint8(r.UintN(16))
			afe.DTSNextAccessUnit = NewClockReference(uint64(r.Uint64N(1<<33)), 0)
		}
		if afe.HasAFDescriptors = r.UintN(2) == 1; afe.HasAFDescriptors {
			afe.AFDescriptors = randBytes(r, int(r.UintN(8)))
		}
		af.AdaptationExtensionField = afe
	}
	return af
}

func TestRoundtripAdaptationField(t *testing.T) {
	r := roundtripRand()
	scratch1 := make([]byte, PacketSize)
	scratch2 := make([]byte, PacketSize)
	for i := 0; i < roundtripIterations; i++ {
		af := randAdaptationField(r)
		n1, err := af.Put(scratch1)
		require.NoError(t, err, "iteration %d", i)

		var parsed PacketAdaptationField
		parsed.Reset()
		pn, err := parsed.Parse(scratch1[:n1])
		require.NoError(t, err, "iteration %d", i)
		assert.Equal(t, n1, pn, "iteration %d", i)

		n2, err := parsed.Put(scratch2)
		require.NoError(t, err, "iteration %d", i)
		require.Equal(t, n1, n2, "iteration %d", i)
		assert.Equal(t, scratch1[:n1], scratch2[:n2], "iteration %d", i)
	}
}

func TestRoundtripClockCodecs(t *testing.T) {
	r := roundtripRand()
	var bs [8]byte
	for i := 0; i < roundtripIterations; i++ {
		cr := NewClockReference(uint64(r.Uint64N(1<<33)), uint64(r.UintN(300)))

		cr.PutPCR(bs[:])
		var pcr ClockReference
		_, err := pcr.ParsePCR(bs[:])
		require.NoError(t, err)
		assert.Equal(t, cr, pcr, "PCR iteration %d", i)

		base := NewClockReference(cr.Base(), 0)
		base.PutPTSDTS(bs[:], 0b0010)
		var pts ClockReference
		_, err = pts.ParsePTSDTS(bs[:])
		require.NoError(t, err)
		assert.Equal(t, base, pts, "PTSDTS iteration %d", i)

		cr.PutESCR(bs[:])
		var escr ClockReference
		_, err = escr.ParseESCR(bs[:])
		require.NoError(t, err)
		assert.Equal(t, cr, escr, "ESCR iteration %d", i)
	}
}
