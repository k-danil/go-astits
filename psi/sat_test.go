package psi

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v3/ts"
)

// Vectors spell the field widths out instead of reusing the parser's constants: one built from the constant under test moves with it and catches nothing.

func satUnit(satelliteTableID uint8, tableCount uint16, body []byte) []byte {
	sectionLength := psiSyntaxHeaderLen + len(body) + crc32Len
	ext := uint16(satelliteTableID)<<10 | tableCount&0x3ff
	sec := []byte{
		0x4d,
		0xf0 | byte(sectionLength>>8)&0x0f, byte(sectionLength),
		byte(ext >> 8), byte(ext),
		0xc0 | 1<<1 | 1, // reserved, version_number 1, current_next_indicator 1
		0x00, 0x00,      // section_number, last_section_number
	}
	sec = append(sec, body...)
	sec = binary.BigEndian.AppendUint32(sec, ts.ComputeCRC32(sec))
	return append([]byte{0x00}, sec...)
}

func satNCRBytes(base uint64, ext uint16) []byte {
	v := base<<15 | uint64(ext) // base 33 bits, reserved 6, ext 9
	return []byte{byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

func satPositionV2TLEBody() []byte {
	body := []byte{
		0x00,             // reserved_zero_future_use
		0x00, 0x00, 0x01, // satellite_id
		0x01,       // reserved(7), position_system = 1
		0x18,       // epoch_year = 24
		0x01, 0x23, // day_of_the_year
	}
	for v := uint32(1); v <= 10; v++ { // Table 11c: day_fraction through mean_motion
		body = binary.BigEndian.AppendUint32(body, v)
	}
	return body
}

func satBeamhoppingBitMapBody() []byte {
	entry := []byte{0x0f, 0x0e, 0x0d, 0x0c} // beamhopping_time_plan_id
	entry = append(entry, 0x00, 0x19)       // reserved(4), beamhopping_time_plan_length = 25
	entry = append(entry, 0x01)             // reserved(6), time_plan_mode = 1
	entry = append(entry, satNCRBytes(1, 2)...)
	entry = append(entry, satNCRBytes(3, 4)...)
	entry = append(entry, 0x00, 0x0b) // reserved(1), bit_map_size = 11
	entry = append(entry, 0x7f, 0xff) // reserved(1), current_slot = 32767
	entry = append(entry, 0xb1, 0xa0) // 1011 0001 101 + 5 padding bits
	return append([]byte{0x00}, entry...)
}

func satBeamhoppingGridBody() []byte {
	grid := []byte{0x00, 0x00, 0x00, 0x02}
	grid = append(grid, 0x00, 43, 0x02) // beamhopping_time_plan_length = 43, time_plan_mode = 2
	grid = append(grid, satNCRBytes(20, 21)...)
	grid = append(grid, satNCRBytes(22, 23)...)
	grid = append(grid, satNCRBytes(24, 25)...) // grid_size
	grid = append(grid, satNCRBytes(26, 27)...) // revisit_duration
	grid = append(grid, satNCRBytes(28, 29)...) // sleep_time
	grid = append(grid, satNCRBytes(30, 31)...) // sleep_duration
	return append([]byte{0x00}, grid...)
}

func satPositionV3Body() []byte {
	body := []byte{
		0x00,             // reserved_zero_future_use
		0x12,             // oem_version_major 1, oem_version_minor 2
		0x18, 0x01, 0x6e, // creation_date_year 24, reserved(7), creation_date_day 366
	}
	body = binary.BigEndian.AppendUint32(body, 0x40000000)
	body = append(body, 0x00, 0xff, 0xee) // satellite_id
	body = append(body, 0x1a)             // reserved(3), metadata 1, usable_start 1, usable_stop 0, accel 1, covariance 0
	body = append(body, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00)
	body = append(body, 0x02, 0x00, 0x02, 0x11, 0x22, 0x33, 0x44)
	body = append(body, 0x55) // reserved(1), interpolation_flag 1, interpolation_type 2, interpolation_degree 5
	body = append(body, 0x03, 0x00, 0x03, 0xde, 0xad, 0xbe, 0xef)
	body = append(body, 0x00, 0x01) // ephemeris_data_count
	body = append(body, 0x04, 0x00, 0x04, 0x00, 0x11, 0x22, 0x33)
	for v := uint32(1); v <= 9; v++ { // Table 11h: state vector of 6 plus 3 acceleration elements
		body = binary.BigEndian.AppendUint32(body, v)
	}
	return body
}

func satPositionV3CovarianceBody() []byte {
	body := []byte{
		0x00,             // reserved_zero_future_use
		0x00,             // oem_version_major 0, oem_version_minor 0
		0x00, 0x00, 0x00, // creation_date_year, reserved(7), creation_date_day
		0x00, 0x00, 0x00, 0x00, // creation_date_day_fraction
		0x00, 0x00, 0x01, // satellite_id
		0x01,       // covariance_flag only
		0x00, 0x00, // ephemeris_data_count = 0
		0x09, 0x00, 0x09, 0x00, 0x00, 0x00, 0x00, // covariance_epoch
	}
	for v := uint32(1); v <= 21; v++ { // Table 11h: the lower triangle of the 6x6 matrix
		body = binary.BigEndian.AppendUint32(body, v)
	}
	return body
}

func satCovarianceSequence() (es [SATCovarianceElementCount]SATFloat32Bits) {
	for k := range es {
		es[k] = SATFloat32Bits(k + 1)
	}
	return
}

var satVectors = []struct {
	name                     string
	satelliteTableID         uint8
	tableCount               uint16
	body                     []byte
	want                     *SAT
	appendNormalizesReserved bool
	wantErr                  error
}{
	{
		name:             "position_v2 TLE element order",
		satelliteTableID: uint8(SatelliteTableIDPositionV2),
		body:             satPositionV2TLEBody(),
		want: &SAT{
			SatelliteTableID: SatelliteTableIDPositionV2,
			SatellitePositionV2Info: []SATSatellitePositionV2{{
				SatelliteID:                      1,
				PositionSystem:                   SATPositionSystemEarthOrbiting,
				EpochYear:                        24,
				DayOfTheYear:                     0x123,
				DayFraction:                      1,
				MeanMotionFirstDerivative:        2,
				MeanMotionSecondDerivative:       3,
				DragTerm:                         4,
				Inclination:                      5,
				RightAscensionOfTheAscendingNode: 6,
				Eccentricity:                     7,
				ArgumentOfPerigee:                8,
				MeanAnomaly:                      9,
				MeanMotion:                       10,
			}},
		},
	},
	{
		name:             "cell_fragment first occurrence",
		satelliteTableID: uint8(SatelliteTableIDCellFragment),
		body: append(append([]byte{
			0x00,                   // reserved_zero_future_use
			0x11, 0x22, 0x33, 0x44, // cell_fragment_id
			0x82,       // first 1, last 0, reserved(4), center_latitude[17:16] = 0b10
			0xa0, 0x70, // center_latitude[15:0]
			0x06,       // reserved(5), center_longitude[18:16] = 0b110
			0x1d, 0xc0, // center_longitude[15:0]
			0x01, 0x86, 0xa0, // max_distance = 100000
			0x00, 0x02, // reserved(6), delivery_system_id_loop_count = 2
			0xaa, 0xaa, 0xaa, 0xaa,
			0xbb, 0xbb, 0xbb, 0xbb,
			0x00, 0x01, // reserved(6), new_delivery_system_id_loop_count = 1
			0xcc, 0xcc, 0xcc, 0xcc,
		}, satNCRBytes(1<<32|1, 299)...), 0x00, 0x00), // obsolescent_delivery_system_id_loop_count = 0
		want: &SAT{
			SatelliteTableID: SatelliteTableIDCellFragment,
			CellFragmentInfo: []SATCellFragment{{
				CellFragmentID:     0x11223344,
				FirstOccurrence:    true,
				CenterLatitude:     -90000,
				CenterLongitude:    -123456,
				MaxDistance:        100000,
				DeliverySystemIDs:  []uint32{0xaaaaaaaa, 0xbbbbbbbb},
				NewDeliverySystems: []SATNewDeliverySystem{{NewDeliverySystemID: 0xcccccccc, TimeOfApplication: SATNCR{Base: 1<<32 | 1, Ext: 299}}},
			}},
		},
	},
	{
		name:             "time_association with leap seconds",
		satelliteTableID: uint8(SatelliteTableIDTimeAssociation),
		body: append(append(append([]byte{
			0x00,
			0x1a, // association_type 1, leap59 1, leap61 0, pastleap59 1, pastleap61 0
		}, satNCRBytes(0x123456789, 0xab)...),
			binary.BigEndian.AppendUint64(nil, 0x0000000065432100)...),
			binary.BigEndian.AppendUint32(nil, 999999999)...),
		want: &SAT{
			SatelliteTableID: SatelliteTableIDTimeAssociation,
			TimeAssociationInfo: SATTimeAssociation{
				AssociationType:                 SATAssociationTypeUTCLeapSeconds,
				Leap59:                          true,
				PastLeap59:                      true,
				NCR:                             SATNCR{Base: 0x123456789, Ext: 0xab},
				AssociationTimestampSeconds:     0x65432100,
				AssociationTimestampNanoseconds: 999999999,
			},
		},
	},
	{
		name:             "beamhopping bit map",
		satelliteTableID: uint8(SatelliteTableIDBeamhoppingTimePlan),
		body:             satBeamhoppingBitMapBody(),
		want: &SAT{
			SatelliteTableID: SatelliteTableIDBeamhoppingTimePlan,
			BeamhoppingTimePlanInfo: []SATBeamhoppingTimePlan{{
				BeamhoppingTimePlanID: 0x0f0e0d0c,
				TimePlanMode:          SATTimePlanModeBitMap,
				TimeOfApplication:     SATNCR{Base: 1, Ext: 2},
				CycleDuration:         SATNCR{Base: 3, Ext: 4},
				CurrentSlot:           0x7fff,
				SlotTransmissionOn:    []bool{true, false, true, true, false, false, false, true, true, false, true},
			}},
		},
	},
	{
		name:             "beamhopping grid",
		satelliteTableID: uint8(SatelliteTableIDBeamhoppingTimePlan),
		body:             satBeamhoppingGridBody(),
		want: &SAT{
			SatelliteTableID: SatelliteTableIDBeamhoppingTimePlan,
			BeamhoppingTimePlanInfo: []SATBeamhoppingTimePlan{{
				BeamhoppingTimePlanID: 2,
				TimePlanMode:          SATTimePlanModeGrid,
				TimeOfApplication:     SATNCR{Base: 20, Ext: 21},
				CycleDuration:         SATNCR{Base: 22, Ext: 23},
				GridSize:              SATNCR{Base: 24, Ext: 25},
				RevisitDuration:       SATNCR{Base: 26, Ext: 27},
				SleepTime:             SATNCR{Base: 28, Ext: 29},
				SleepDuration:         SATNCR{Base: 30, Ext: 31},
			}},
		},
	},
	{
		name:             "position_v3 metadata and acceleration",
		satelliteTableID: uint8(SatelliteTableIDPositionV3),
		body:             satPositionV3Body(),
		want: &SAT{
			SatelliteTableID: SatelliteTableIDPositionV3,
			SatellitePositionV3Info: SATSatellitePositionV3{
				OEMVersionMajor: 1,
				OEMVersionMinor: 2,
				CreationDate:    SATYearDayTime{Year: 24, Day: 366, DayFraction: 0x40000000},
				Satellites: []SATSatelliteEphemeris{{
					SatelliteID:         0x00ffee,
					MetadataFlag:        true,
					UsableStartTimeFlag: true,
					EphemerisAccelFlag:  true,
					Metadata: SATEphemerisMetadata{
						TotalStartTime:      SATYearDayTime{Year: 1, Day: 1},
						TotalStopTime:       SATYearDayTime{Year: 2, Day: 2, DayFraction: 0x11223344},
						InterpolationFlag:   true,
						InterpolationType:   SATInterpolationTypeLagrange,
						InterpolationDegree: 5,
						UsableStartTime:     SATYearDayTime{Year: 3, Day: 3, DayFraction: 0xdeadbeef},
					},
					EphemerisData: []SATEphemerisRecord{{
						Epoch:          SATYearDayTime{Year: 4, Day: 4, DayFraction: 0x00112233},
						EphemerisX:     1,
						EphemerisY:     2,
						EphemerisZ:     3,
						EphemerisXDot:  4,
						EphemerisYDot:  5,
						EphemerisZDot:  6,
						EphemerisXDdot: 7,
						EphemerisYDdot: 8,
						EphemerisZDdot: 9,
					}},
				}},
			},
		},
	},
	{
		name:             "position_v3 covariance",
		satelliteTableID: uint8(SatelliteTableIDPositionV3),
		body:             satPositionV3CovarianceBody(),
		want: &SAT{
			SatelliteTableID: SatelliteTableIDPositionV3,
			SatellitePositionV3Info: SATSatellitePositionV3{
				Satellites: []SATSatelliteEphemeris{{
					SatelliteID:    1,
					CovarianceFlag: true,
					Covariance: SATCovariance{
						CovarianceEpoch:    SATYearDayTime{Year: 9, Day: 9},
						CovarianceElements: satCovarianceSequence(),
					},
				}},
			},
		},
	},
	{
		name:             "reserved satellite_table_id keeps its body",
		satelliteTableID: 63,
		tableCount:       0x3ff,
		body:             []byte{0x00, 0xde, 0xad, 0xbe, 0xef},
		want: &SAT{
			SatelliteTableID: 63,
			TableCount:       0x3ff,
			Reserved:         []byte{0xde, 0xad, 0xbe, 0xef},
		},
	},
	{
		name:             "reserved ones: year_day_time day stays 9 bits",
		satelliteTableID: uint8(SatelliteTableIDPositionV3),
		body: []byte{
			0x00,
			0x00,             // oem_version_major 0, oem_version_minor 0
			0x00, 0xff, 0x00, // creation_date_year 0, reserved(7) all ones, creation_date_day = 256
			0x00, 0x00, 0x00, 0x00, // creation_date_day_fraction
			0x00, 0x00, 0x01, // satellite_id
			0x00,       // no flags
			0x00, 0x00, // ephemeris_data_count = 0
		},
		want: &SAT{
			SatelliteTableID: SatelliteTableIDPositionV3,
			SatellitePositionV3Info: SATSatellitePositionV3{
				CreationDate: SATYearDayTime{Day: 256},
				Satellites:   []SATSatelliteEphemeris{{SatelliteID: 1}},
			},
		},
		appendNormalizesReserved: true,
	},
	{
		name:             "reserved ones: delivery_system_id_loop_count stays 10 bits",
		satelliteTableID: uint8(SatelliteTableIDCellFragment),
		body: []byte{
			0x00,
			0x00, 0x00, 0x00, 0x01, // cell_fragment_id
			0x3c, 0x01, // first 0, last 0, reserved(4) all ones, delivery_system_id_loop_count = 1
			0xde, 0xad, 0xbe, 0xef,
			0xfc, 0x00, // reserved(6) all ones, new_delivery_system_id_loop_count = 0
			0xfc, 0x00, // reserved(6) all ones, obsolescent_delivery_system_id_loop_count = 0
		},
		want: &SAT{
			SatelliteTableID: SatelliteTableIDCellFragment,
			CellFragmentInfo: []SATCellFragment{{
				CellFragmentID:    1,
				DeliverySystemIDs: []uint32{0xdeadbeef},
			}},
		},
		appendNormalizesReserved: true,
	},
	{
		name:             "reserved ones: time_plan_mode stays 2 bits",
		satelliteTableID: uint8(SatelliteTableIDBeamhoppingTimePlan),
		body: append(append(append(append([]byte{
			0x00,
			0x00, 0x00, 0x00, 0x01, // beamhopping_time_plan_id
			0xf0, 31, // reserved(4) all ones, beamhopping_time_plan_length = 31
			0xfc, // reserved(6) all ones, time_plan_mode = 0
		}, satNCRBytes(1, 1)...), satNCRBytes(2, 2)...), satNCRBytes(3, 3)...), satNCRBytes(4, 4)...),
		want: &SAT{
			SatelliteTableID: SatelliteTableIDBeamhoppingTimePlan,
			BeamhoppingTimePlanInfo: []SATBeamhoppingTimePlan{{
				BeamhoppingTimePlanID: 1,
				TimePlanMode:          SATTimePlanModeDwell,
				TimeOfApplication:     SATNCR{Base: 1, Ext: 1},
				CycleDuration:         SATNCR{Base: 2, Ext: 2},
				DwellDuration:         SATNCR{Base: 3, Ext: 3},
				OnTime:                SATNCR{Base: 4, Ext: 4},
			}},
		},
		appendNormalizesReserved: true,
	},
	{
		name:             "reserved ones: position_system stays the low bit",
		satelliteTableID: uint8(SatelliteTableIDPositionV2),
		body: []byte{
			0x00,
			0x0a, 0xbc, 0xde, // satellite_id
			0xfe,       // reserved(7) all ones, position_system = 0
			0x01, 0x92, // orbital position
			0x80, // west_east_flag = 1, reserved(7)
		},
		want: &SAT{
			SatelliteTableID: SatelliteTableIDPositionV2,
			SatellitePositionV2Info: []SATSatellitePositionV2{{
				SatelliteID:     0x0abcde,
				OrbitalPosition: 0x0192,
				WestEastFlag:    true,
			}},
		},
		appendNormalizesReserved: true,
	},
	{
		name:             "ephemeris_data_count stays 16 bits",
		satelliteTableID: uint8(SatelliteTableIDPositionV3),
		body: []byte{
			0x00,
			0x00,
			0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x01,
			0x00,
			0xf0, 0x00, // ephemeris_data_count = 61440, far past the section end
		},
		wantErr: ts.ErrInvalidData,
	},
	{
		name:             "beamhopping_time_plan_length shorter than the fields of its mode",
		satelliteTableID: uint8(SatelliteTableIDBeamhoppingTimePlan),
		body: append([]byte{
			0x00,
			0x0f, 0x0e, 0x0d, 0x0c, // beamhopping_time_plan_id
			0x00, 0x05, // reserved_zero_future_use, beamhopping_time_plan_length 5
			0x00, // reserved_zero_future_use, time_plan_mode 0
		}, make([]byte, 24)...), // time_of_application, cycle_duration, dwell_duration, on_time
		wantErr: ErrSATTimePlanLength,
	},
}

func TestParseSATVectors(t *testing.T) {
	for _, tc := range satVectors {
		t.Run(tc.name, func(t *testing.T) {
			unit := satUnit(tc.satelliteTableID, tc.tableCount, tc.body)
			d, err := Parse(unit)
			require.NoError(t, err)
			if tc.wantErr != nil {
				require.NotEmpty(t, d.Errors)
				require.ErrorIs(t, d.Errors[0], tc.wantErr)
				return
			}
			require.Empty(t, d.Errors)
			require.Len(t, d.Sections, 1)
			sat, ok := d.Sections[0].Syntax.Data.(*SAT)
			require.True(t, ok)
			assert.Equal(t, tc.want, sat)

			if tc.appendNormalizesReserved {
				return
			}
			back, err := d.Append(nil)
			require.NoError(t, err)
			assert.Equal(t, unit, back)
		})
	}
}
