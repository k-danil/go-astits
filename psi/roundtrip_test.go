package psi

import (
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v2/descriptor"
)

func randDescriptors(r *rand.Rand) (ds []descriptor.Descriptor) {
	for i := uint(0); i < r.UintN(3); i++ {
		d := &descriptor.StreamIdentifier{
			Header:       descriptor.Header{Tag: descriptor.TagStreamIdentifier},
			ComponentTag: uint8(r.UintN(256)),
		}
		ds = append(ds, d)
	}
	return
}

func randSection(r *rand.Rand, tableID TableID, data SectionSyntaxData, sectionLength int) Section {
	return Section{
		Header: SectionHeader{
			TableID:                tableID,
			SectionSyntaxIndicator: true,
			PrivateBit:             r.UintN(2) == 1,
			SectionLength:          uint16(sectionLength),
		},
		Syntax: &SectionSyntax{
			Data: data,
			Header: SectionSyntaxHeader{
				TableIDExtension:     uint16(r.UintN(1 << 16)),
				VersionNumber:        uint8(r.UintN(32)),
				CurrentNextIndicator: r.UintN(2) == 1,
				SectionNumber:        uint8(r.UintN(256)),
				LastSectionNumber:    uint8(r.UintN(256)),
			},
		},
	}
}

func TestRoundtripPSI(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 8))
	for i := 0; i < 300; i++ {
		pat := &PAT{TransportStreamID: uint16(r.UintN(1 << 16))}
		for j := uint(0); j < 1+r.UintN(10); j++ {
			pat.Programs = append(pat.Programs, PATProgram{
				ProgramMapID:  uint16(r.UintN(1 << 13)),
				ProgramNumber: uint16(r.UintN(1 << 16)),
			})
		}

		pmt := &PMT{
			ProgramNumber:      uint16(r.UintN(1 << 16)),
			PCRPID:             uint16(r.UintN(1 << 13)),
			ProgramDescriptors: randDescriptors(r),
		}
		for j := uint(0); j < 1+r.UintN(5); j++ {
			pmt.ElementaryStreams = append(pmt.ElementaryStreams, ElementaryStream{
				StreamType:                  StreamTypeH264Video,
				ElementaryPID:               uint16(r.UintN(1 << 13)),
				ElementaryStreamDescriptors: randDescriptors(r),
			})
		}

		d := &Data{
			PointerField: int(r.UintN(5)),
			Sections: []Section{
				randSection(r, TableIDPAT, pat, pat.CalcSectionLength()),
				randSection(r, TableIDPMT, pmt, pmt.CalcSectionLength()),
			},
		}

		b1, err := d.Append(nil)
		require.NoError(t, err, "iteration %d", i)

		parsed, err := Parse(b1)
		require.NoError(t, err, "iteration %d", i)
		require.Len(t, parsed.Sections, 2, "iteration %d", i)

		b2, err := parsed.Append(nil)
		require.NoError(t, err, "iteration %d", i)
		assert.Equal(t, b1, b2, "iteration %d", i)
	}
}
