package psi

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v2/descriptor"
)

// randDVBTime returns a UTC time at DVB resolution (whole seconds) in a range
// whose 16-bit MJD does not wrap, so it round-trips exactly.
func randDVBTime(r *rand.Rand) time.Time {
	return time.Date(2000+int(r.UintN(30)), time.Month(1+r.UintN(12)), 1+int(r.UintN(28)),
		int(r.UintN(24)), int(r.UintN(60)), int(r.UintN(60)), 0, time.UTC)
}

func randDescriptors(r *rand.Rand) (ds []descriptor.Descriptor) {
	for i := uint(0); i < r.UintN(3); i++ {
		d := &descriptor.StreamIdentifier{
			Header:       descriptor.Header{Tag: descriptor.TagStreamIdentifier},
			ComponentTag: uint8(r.UintN(256)),
		}
		d.Header.Length = uint8(d.CalcLength()) // parse fills Length; match it for the semantic check
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

func randRST(r *rand.Rand) *RST {
	rst := &RST{}
	for j := uint(0); j < 1+r.UintN(5); j++ {
		rst.Events = append(rst.Events, RSTEvent{
			TransportStreamID: uint16(r.UintN(1 << 16)),
			OriginalNetworkID: uint16(r.UintN(1 << 16)),
			ServiceID:         uint16(r.UintN(1 << 16)),
			EventID:           uint16(r.UintN(1 << 16)),
			RunningStatus:     uint8(r.UintN(8)),
		})
	}
	return rst
}

func TestRoundtripPSITrivial(t *testing.T) {
	r := rand.New(rand.NewPCG(11, 12))
	for i := 0; i < 300; i++ {
		cases := []struct {
			tableID TableID
			data    SectionSyntaxData
		}{
			{TableIDST, &ST{}},
			{TableIDDIT, &DIT{TransitionFlag: r.UintN(2) == 1}},
			{TableIDRST, randRST(r)},
			{TableIDTSDT, &TSDT{Descriptors: randDescriptors(r)}},
			{TableIDTDT, &TDT{UTCTime: randDVBTime(r)}},
			{TableIDTOT, &TOT{UTCTime: randDVBTime(r), Descriptors: randDescriptors(r)}},
		}
		for _, tc := range cases {
			d := &Data{
				PointerField: int(r.UintN(5)),
				Sections:     []Section{randSection(r, tc.tableID, tc.data, tc.data.(sectionBody).CalcSectionLength())},
			}

			b1, err := d.Append(nil)
			require.NoError(t, err, "%T", tc.data)

			parsed, err := Parse(b1)
			require.NoError(t, err, "%T", tc.data)
			require.Len(t, parsed.Sections, 1, "%T", tc.data)

			b2, err := parsed.Append(nil)
			require.NoError(t, err, "%T", tc.data)
			assert.Equal(t, b1, b2, "%T byte-stable", tc.data)
			// A zero-length section (ST) parses back with no syntax — only byte stability applies.
			if parsed.Sections[0].Syntax != nil {
				assert.Equal(t, tc.data, parsed.Sections[0].Syntax.Data, "%T semantic", tc.data)
			}
		}
	}
}

func randDuration(r *rand.Rand) time.Duration {
	return time.Duration(r.UintN(24))*time.Hour + time.Duration(r.UintN(60))*time.Minute + time.Duration(r.UintN(60))*time.Second
}

func TestRoundtripPSITables(t *testing.T) {
	r := rand.New(rand.NewPCG(13, 14))
	for i := 0; i < 300; i++ {
		// ext is the section's TableIDExtension; SDT/EIT/NIT/BAT mirror it into an ID field.
		ext := uint16(r.UintN(1 << 16))

		sdt := &SDT{TransportStreamID: ext, OriginalNetworkID: uint16(r.UintN(1 << 16))}
		for j := uint(0); j < 1+r.UintN(4); j++ {
			sdt.Services = append(sdt.Services, SDTService{
				ServiceID: uint16(r.UintN(1 << 16)), HasEITSchedule: r.UintN(2) == 1,
				HasEITPresentFollowing: r.UintN(2) == 1, HasFreeCSAMode: r.UintN(2) == 1,
				RunningStatus: uint8(r.UintN(8)), Descriptors: randDescriptors(r),
			})
		}

		eit := &EIT{ServiceID: ext, TransportStreamID: uint16(r.UintN(1 << 16)),
			OriginalNetworkID: uint16(r.UintN(1 << 16)), SegmentLastSectionNumber: uint8(r.UintN(256)), LastTableID: uint8(r.UintN(256))}
		for j := uint(0); j < 1+r.UintN(4); j++ {
			eit.Events = append(eit.Events, EITEvent{
				EventID: uint16(r.UintN(1 << 16)), StartTime: randDVBTime(r), Duration: randDuration(r),
				RunningStatus: uint8(r.UintN(8)), HasFreeCSAMode: r.UintN(2) == 1, Descriptors: randDescriptors(r),
			})
		}

		nit := &NIT{NetworkID: ext, NetworkDescriptors: randDescriptors(r)}
		bat := &BAT{BouquetID: ext, BouquetDescriptors: randDescriptors(r)}
		for j := uint(0); j < 1+r.UintN(3); j++ {
			nit.TransportStreams = append(nit.TransportStreams, NITTransportStream{TransportStreamID: uint16(r.UintN(1 << 16)), OriginalNetworkID: uint16(r.UintN(1 << 16)), TransportDescriptors: randDescriptors(r)})
			bat.TransportStreams = append(bat.TransportStreams, BATTransportStream{TransportStreamID: uint16(r.UintN(1 << 16)), OriginalNetworkID: uint16(r.UintN(1 << 16)), TransportDescriptors: randDescriptors(r)})
		}

		sit := &SIT{TransmissionInfoDescriptors: randDescriptors(r)}
		for j := uint(0); j < 1+r.UintN(4); j++ {
			sit.Services = append(sit.Services, SITService{ServiceID: uint16(r.UintN(1 << 16)), RunningStatus: uint8(r.UintN(8)), Descriptors: randDescriptors(r)})
		}

		iso := &ISO14496Section{}
		for j := uint(0); j < 1+r.UintN(10); j++ {
			iso.Data = append(iso.Data, uint8(r.UintN(256)))
		}

		cases := []struct {
			tableID TableID
			data    SectionSyntaxData
		}{
			{TableIDCAT, &CAT{Descriptors: randDescriptors(r)}},
			{TableIDSDTVariant1, sdt},
			{TableIDEITStart, eit},
			{TableIDNITVariant1, nit},
			{TableIDBAT, bat},
			{TableIDSIT, sit},
			{TableIDISO14496, iso},
		}
		for _, tc := range cases {
			sec := randSection(r, tc.tableID, tc.data, tc.data.(sectionBody).CalcSectionLength())
			sec.Syntax.Header.TableIDExtension = ext
			d := &Data{PointerField: int(r.UintN(5)), Sections: []Section{sec}}

			b1, err := d.Append(nil)
			require.NoError(t, err, "%T", tc.data)
			parsed, err := Parse(b1)
			require.NoError(t, err, "%T", tc.data)
			require.Len(t, parsed.Sections, 1, "%T", tc.data)
			b2, err := parsed.Append(nil)
			require.NoError(t, err, "%T", tc.data)
			assert.Equal(t, b1, b2, "%T byte-stable", tc.data)
			assert.Equal(t, tc.data, parsed.Sections[0].Syntax.Data, "%T semantic", tc.data)
		}
	}
}

func TestRoundtripPSIMetadata(t *testing.T) {
	r := rand.New(rand.NewPCG(21, 22))
	for i := 0; i < 300; i++ {
		md := &Metadata{
			MetadataServiceID:         uint8(r.UintN(256)),
			SectionFragmentIndication: uint8(r.UintN(4)),
			VersionNumber:             uint8(r.UintN(32)),
			CurrentNextIndicator:      r.UintN(2) == 1,
			SectionNumber:             uint8(r.UintN(256)),
			LastSectionNumber:         uint8(r.UintN(256)),
		}
		for j := uint(0); j < 1+r.UintN(20); j++ {
			md.MetadataBytes = append(md.MetadataBytes, uint8(r.UintN(256)))
		}

		sec := Section{
			Header: SectionHeader{
				TableID:                TableIDMetadata,
				SectionSyntaxIndicator: true,
				PrivateBit:             r.UintN(2) == 1,
				RandomAccessIndicator:  r.UintN(2) == 1,
				DecoderConfigFlag:      r.UintN(2) == 1,
			},
			Syntax: &SectionSyntax{Data: md},
		}
		d := &Data{PointerField: int(r.UintN(5)), Sections: []Section{sec}}

		b1, err := d.Append(nil)
		require.NoError(t, err)
		parsed, err := Parse(b1)
		require.NoError(t, err)
		require.Len(t, parsed.Sections, 1)
		b2, err := parsed.Append(nil)
		require.NoError(t, err)
		assert.Equal(t, b1, b2, "byte-stable")
		assert.Equal(t, md, parsed.Sections[0].Syntax.Data, "metadata semantic")
		assert.Equal(t, sec.Header.RandomAccessIndicator, parsed.Sections[0].Header.RandomAccessIndicator)
		assert.Equal(t, sec.Header.DecoderConfigFlag, parsed.Sections[0].Header.DecoderConfigFlag)
	}
}
