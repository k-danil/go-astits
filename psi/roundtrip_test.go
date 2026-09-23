package psi

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v3/descriptor"
)

// randDVBTime returns a UTC time at DVB resolution (whole seconds) in a range
// whose 16-bit MJD does not wrap, so it round-trips exactly.
func randDVBTime(r *rand.Rand) time.Time {
	return time.Date(2000+int(r.UintN(30)), time.Month(1+r.UintN(12)), 1+int(r.UintN(28)),
		int(r.UintN(24)), int(r.UintN(60)), int(r.UintN(60)), 0, time.UTC)
}

func randDescriptors(r *rand.Rand) (ds []descriptor.Descriptor) {
	for range r.UintN(3) {
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
	for i := range 300 {
		pat := &PAT{TransportStreamID: uint16(r.UintN(1 << 16))}
		for range 1 + r.UintN(10) {
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
		for range 1 + r.UintN(5) {
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
	for range 1 + r.UintN(5) {
		rst.Events = append(rst.Events, RSTEvent{
			TransportStreamID: uint16(r.UintN(1 << 16)),
			OriginalNetworkID: uint16(r.UintN(1 << 16)),
			ServiceID:         uint16(r.UintN(1 << 16)),
			EventID:           uint16(r.UintN(1 << 16)),
			RunningStatus:     RunningStatus(r.UintN(8)),
		})
	}
	return rst
}

func TestRoundtripPSITrivial(t *testing.T) {
	r := rand.New(rand.NewPCG(11, 12))
	for range 300 {
		cases := []struct {
			tableID TableID
			data    sectionBody
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
				Sections:     []Section{randSection(r, tc.tableID, tc.data, tc.data.CalcSectionLength())},
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
	for range 300 {
		// ext is the section's TableIDExtension; SDT/EIT/NIT/BAT mirror it into an ID field.
		ext := uint16(r.UintN(1 << 16))

		sdt := &SDT{TransportStreamID: ext, OriginalNetworkID: uint16(r.UintN(1 << 16))}
		for range 1 + r.UintN(4) {
			sdt.Services = append(sdt.Services, SDTService{
				ServiceID: uint16(r.UintN(1 << 16)), HasEITSchedule: r.UintN(2) == 1,
				HasEITPresentFollowing: r.UintN(2) == 1, HasFreeCSAMode: r.UintN(2) == 1,
				RunningStatus: RunningStatus(r.UintN(8)), Descriptors: randDescriptors(r),
			})
		}

		eit := &EIT{ServiceID: ext, TransportStreamID: uint16(r.UintN(1 << 16)),
			OriginalNetworkID: uint16(r.UintN(1 << 16)), SegmentLastSectionNumber: uint8(r.UintN(256)), LastTableID: TableID(r.UintN(256))}
		for range 1 + r.UintN(4) {
			eit.Events = append(eit.Events, EITEvent{
				EventID: uint16(r.UintN(1 << 16)), StartTime: randDVBTime(r), Duration: randDuration(r),
				RunningStatus: RunningStatus(r.UintN(8)), HasFreeCSAMode: r.UintN(2) == 1, Descriptors: randDescriptors(r),
			})
		}

		nit := &NIT{NetworkID: ext, NetworkDescriptors: randDescriptors(r)}
		bat := &BAT{BouquetID: ext, BouquetDescriptors: randDescriptors(r)}
		for range 1 + r.UintN(3) {
			nit.TransportStreams = append(nit.TransportStreams, NITTransportStream{TransportStreamID: uint16(r.UintN(1 << 16)), OriginalNetworkID: uint16(r.UintN(1 << 16)), TransportDescriptors: randDescriptors(r)})
			bat.TransportStreams = append(bat.TransportStreams, BATTransportStream{TransportStreamID: uint16(r.UintN(1 << 16)), OriginalNetworkID: uint16(r.UintN(1 << 16)), TransportDescriptors: randDescriptors(r)})
		}

		sit := &SIT{TransmissionInfoDescriptors: randDescriptors(r)}
		for range 1 + r.UintN(4) {
			sit.Services = append(sit.Services, SITService{ServiceID: uint16(r.UintN(1 << 16)), RunningStatus: RunningStatus(r.UintN(8)), Descriptors: randDescriptors(r)})
		}

		iso := &ISO14496Section{}
		for range 1 + r.UintN(10) {
			iso.Data = append(iso.Data, uint8(r.UintN(256)))
		}

		cases := []struct {
			tableID TableID
			data    sectionBody
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
			sec := randSection(r, tc.tableID, tc.data, tc.data.CalcSectionLength())
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
	for range 300 {
		md := &Metadata{
			MetadataServiceID:         uint8(r.UintN(256)),
			SectionFragmentIndication: uint8(r.UintN(4)),
			VersionNumber:             uint8(r.UintN(32)),
			CurrentNextIndicator:      r.UintN(2) == 1,
			SectionNumber:             uint8(r.UintN(256)),
			LastSectionNumber:         uint8(r.UintN(256)),
		}
		for range 1 + r.UintN(20) {
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

func randSATNCR(r *rand.Rand) SATNCR {
	return SATNCR{Base: r.Uint64N(1 << 33), Ext: uint16(r.UintN(1 << satNCRExtBits))}
}

func randSATYearDayTime(r *rand.Rand) SATYearDayTime {
	return SATYearDayTime{Year: uint8(r.UintN(100)), Day: uint16(1 + r.UintN(366)), DayFraction: SATFloat32Bits(r.Uint32())}
}

func randSATPositionV2(r *rand.Rand) (d *SAT) {
	d = &SAT{SatelliteTableID: SatelliteTableIDPositionV2}
	for k := range 2 + r.UintN(3) {
		s := SATSatellitePositionV2{SatelliteID: uint32(r.UintN(1 << 24)), PositionSystem: SATPositionSystem(k % 2)}
		if s.PositionSystem == SATPositionSystemEarthOrbiting {
			s.EpochYear = uint8(r.UintN(100))
			s.DayOfTheYear = uint16(r.UintN(1 << 16))
			for _, e := range s.tleElements() {
				*e = SATFloat32Bits(r.Uint32())
			}
		} else {
			s.OrbitalPosition = uint16(r.UintN(1 << 16))
			s.WestEastFlag = r.UintN(2) == 1
		}
		d.SatellitePositionV2Info = append(d.SatellitePositionV2Info, s)
	}
	return
}

func randSATCellFragment(r *rand.Rand) (d *SAT) {
	d = &SAT{SatelliteTableID: SatelliteTableIDCellFragment}
	for k := range 2 + r.UintN(3) {
		c := SATCellFragment{CellFragmentID: r.Uint32(), FirstOccurrence: k%2 == 0, LastOccurrence: r.UintN(2) == 1}
		if c.FirstOccurrence {
			c.CenterLatitude = int32(r.IntN(1<<satCenterLatitudeBits) - 1<<(satCenterLatitudeBits-1))
			c.CenterLongitude = int32(r.IntN(1<<satCenterLongitudeBits) - 1<<(satCenterLongitudeBits-1))
			c.MaxDistance = uint32(r.UintN(1 << 24))
		}
		for range r.UintN(4) {
			c.DeliverySystemIDs = append(c.DeliverySystemIDs, r.Uint32())
		}
		for range r.UintN(3) {
			c.NewDeliverySystems = append(c.NewDeliverySystems, SATNewDeliverySystem{NewDeliverySystemID: r.Uint32(), TimeOfApplication: randSATNCR(r)})
		}
		for range r.UintN(3) {
			c.ObsolescentDeliverySystems = append(c.ObsolescentDeliverySystems, SATObsolescentDeliverySystem{ObsolescentDeliverySystemID: r.Uint32(), TimeOfObsolescence: randSATNCR(r)})
		}
		d.CellFragmentInfo = append(d.CellFragmentInfo, c)
	}
	return
}

func randSATTimeAssociation(r *rand.Rand) (d *SAT) {
	t := SATTimeAssociation{
		AssociationType:                 SATAssociationType(r.UintN(2)),
		NCR:                             randSATNCR(r),
		AssociationTimestampSeconds:     r.Uint64(),
		AssociationTimestampNanoseconds: r.Uint32(),
	}
	if t.AssociationType == SATAssociationTypeUTCLeapSeconds {
		t.Leap59, t.Leap61 = r.UintN(2) == 1, r.UintN(2) == 1
		t.PastLeap59, t.PastLeap61 = r.UintN(2) == 1, r.UintN(2) == 1
	}
	d = &SAT{SatelliteTableID: SatelliteTableIDTimeAssociation, TimeAssociationInfo: t}
	return
}

func randSATBeamhoppingTimePlan(r *rand.Rand) (d *SAT) {
	d = &SAT{SatelliteTableID: SatelliteTableIDBeamhoppingTimePlan}
	for k := range 4 {
		p := SATBeamhoppingTimePlan{
			BeamhoppingTimePlanID: r.Uint32(),
			TimePlanMode:          SATTimePlanMode(k),
			TimeOfApplication:     randSATNCR(r),
			CycleDuration:         randSATNCR(r),
		}
		switch p.TimePlanMode {
		case SATTimePlanModeDwell:
			p.DwellDuration, p.OnTime = randSATNCR(r), randSATNCR(r)
		case SATTimePlanModeBitMap:
			p.CurrentSlot = uint16(r.UintN(1 << 15))
			for range 1 + r.UintN(40) {
				p.SlotTransmissionOn = append(p.SlotTransmissionOn, r.UintN(2) == 1)
			}
		case SATTimePlanModeGrid:
			p.GridSize, p.RevisitDuration = randSATNCR(r), randSATNCR(r)
			p.SleepTime, p.SleepDuration = randSATNCR(r), randSATNCR(r)
		default:
			for range 1 + r.UintN(5) {
				p.Reserved = append(p.Reserved, uint8(r.UintN(256)))
			}
		}
		d.BeamhoppingTimePlanInfo = append(d.BeamhoppingTimePlanInfo, p)
	}
	return
}

func randSATPositionV3(r *rand.Rand) (d *SAT) {
	v3 := SATSatellitePositionV3{
		OEMVersionMajor: uint8(r.UintN(16)),
		OEMVersionMinor: uint8(r.UintN(16)),
		CreationDate:    randSATYearDayTime(r),
	}
	for range 1 + r.UintN(3) {
		s := SATSatelliteEphemeris{
			SatelliteID:         uint32(r.UintN(1 << 24)),
			MetadataFlag:        r.UintN(2) == 1,
			UsableStartTimeFlag: r.UintN(2) == 1,
			UsableStopTimeFlag:  r.UintN(2) == 1,
			EphemerisAccelFlag:  r.UintN(2) == 1,
			CovarianceFlag:      r.UintN(2) == 1,
		}
		if s.MetadataFlag {
			s.Metadata = SATEphemerisMetadata{
				TotalStartTime:      randSATYearDayTime(r),
				TotalStopTime:       randSATYearDayTime(r),
				InterpolationFlag:   r.UintN(2) == 1,
				InterpolationType:   SATInterpolationType(r.UintN(8)),
				InterpolationDegree: uint8(r.UintN(8)),
			}
			if s.UsableStartTimeFlag {
				s.Metadata.UsableStartTime = randSATYearDayTime(r)
			}
			if s.UsableStopTimeFlag {
				s.Metadata.UsableStopTime = randSATYearDayTime(r)
			}
		}
		for range r.UintN(3) {
			rec := SATEphemerisRecord{Epoch: randSATYearDayTime(r)}
			for _, f := range rec.stateVector() {
				*f = SATFloat32Bits(r.Uint32())
			}
			if s.EphemerisAccelFlag {
				for _, f := range rec.acceleration() {
					*f = SATFloat32Bits(r.Uint32())
				}
			}
			s.EphemerisData = append(s.EphemerisData, rec)
		}
		if s.CovarianceFlag {
			s.Covariance.CovarianceEpoch = randSATYearDayTime(r)
			for k := range s.Covariance.CovarianceElements {
				s.Covariance.CovarianceElements[k] = SATFloat32Bits(r.Uint32())
			}
		}
		v3.Satellites = append(v3.Satellites, s)
	}
	d = &SAT{SatelliteTableID: SatelliteTableIDPositionV3, SatellitePositionV3Info: v3}
	return
}

var satGenerators = []struct {
	name string
	gen  func(r *rand.Rand) *SAT
}{
	{"satellite_position_v2_info", randSATPositionV2},
	{"cell_fragment_info", randSATCellFragment},
	{"time_association_info", randSATTimeAssociation},
	{"beamhopping_time_plan_info", randSATBeamhoppingTimePlan},
	{"satellite_position_v3_info", randSATPositionV3},
}

func randSATData(r *rand.Rand, gen func(*rand.Rand) *SAT) (sat *SAT, d *Data) {
	sat = gen(r)
	sat.TableCount = uint16(r.UintN(1 << satTableCountBits))
	sec := randSection(r, TableIDSAT, sat, sat.CalcSectionLength())
	d = &Data{PointerField: int(r.UintN(5)), Sections: []Section{sec}}
	return
}

func TestRoundtripPSISAT(t *testing.T) {
	for _, tc := range satGenerators {
		t.Run(tc.name, func(t *testing.T) {
			r := rand.New(rand.NewPCG(41, 42))
			for range 100 {
				sat, d := randSATData(r, tc.gen)
				b1, err := d.Append(nil)
				require.NoError(t, err)
				parsed, err := Parse(b1)
				require.NoError(t, err)
				require.Empty(t, parsed.Errors)
				require.Len(t, parsed.Sections, 1)
				b2, err := parsed.Append(nil)
				require.NoError(t, err)
				assert.Equal(t, b1, b2, "byte-stable")
				assert.Equal(t, sat, parsed.Sections[0].Syntax.Data, "semantic")
			}
		})
	}
}
