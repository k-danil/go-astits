package psi

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
	"github.com/k-danil/go-astits/v3/internal/errclass"
	"github.com/k-danil/go-astits/v3/ts"
)

type SatelliteTableID uint8

const (
	SatelliteTableIDPositionV2 SatelliteTableID = iota
	SatelliteTableIDCellFragment
	SatelliteTableIDTimeAssociation
	SatelliteTableIDBeamhoppingTimePlan
	SatelliteTableIDPositionV3
)

type SATPositionSystem uint8

const (
	SATPositionSystemGeostationary SATPositionSystem = iota
	SATPositionSystemEarthOrbiting
)

type SATAssociationType uint8

const (
	SATAssociationTypeUTC SATAssociationType = iota
	SATAssociationTypeUTCLeapSeconds
)

type SATTimePlanMode uint8

const (
	SATTimePlanModeDwell SATTimePlanMode = iota
	SATTimePlanModeBitMap
	SATTimePlanModeGrid
)

type SATInterpolationType uint8

const (
	SATInterpolationTypeLinear   SATInterpolationType = 1
	SATInterpolationTypeLagrange SATInterpolationType = 2
	SATInterpolationTypeHermite  SATInterpolationType = 4
)

const SATCovarianceElementCount = 21

var ErrSATTimePlanLength = errclass.New("astits: beamhopping_time_plan_length shorter than the fields of its time_plan_mode", ts.ErrInvalidData)

const (
	satTableCountBits        = 10
	satTableCountMask        = 1<<satTableCountBits - 1
	satReservedZeroFutureUse = 0x00
	satReservedByteLen       = 1

	satUint24Len       = 3
	satSPFLen          = 4
	bitsPerByte        = 8
	satSignExtendWidth = 32
	satByteMSBMask     = 0x80

	satNCRLen          = 6
	satNCRExtBits      = 9
	satNCRReservedBits = 6
	satNCRBaseShift    = satNCRReservedBits + satNCRExtBits
	satNCRExtMask      = 1<<satNCRExtBits - 1
	satNCRBaseBits     = satNCRLen*bitsPerByte - satNCRBaseShift
	satNCRBaseMask     = 1<<satNCRBaseBits - 1

	satYearLen        = 1
	satDayLen         = 2
	satYearDayTimeLen = satYearLen + satDayLen + satSPFLen
	satDayBits        = 9
	satDayMask        = 1<<satDayBits - 1
)

type SATFloat32Bits uint32

func (b SATFloat32Bits) Float32() float32 {
	return math.Float32frombits(uint32(b))
}

// Append writes table_id_extension from SatelliteTableID and TableCount, ignoring Syntax.Header.TableIDExtension.
type SAT struct {
	SatellitePositionV2Info []SATSatellitePositionV2 `json:"_satellite_position_v2_info,omitempty"`
	CellFragmentInfo        []SATCellFragment        `json:"_cell_fragment_info,omitempty"`
	BeamhoppingTimePlanInfo []SATBeamhoppingTimePlan `json:"_beamhopping_time_plan_info,omitempty"`
	Reserved                []byte                   `json:"_reserved,omitempty"`
	TimeAssociationInfo     SATTimeAssociation       `json:"_time_association_info"`
	SatellitePositionV3Info SATSatellitePositionV3   `json:"_satellite_position_v3_info"`
	TableCount              uint16                   `json:"table_count"`
	SatelliteTableID        SatelliteTableID         `json:"satellite_table_id"`
}

type SATNCR struct {
	Base uint64 `json:"base"`
	Ext  uint16 `json:"ext"`
}

// Day is the day of the year.
type SATYearDayTime struct {
	DayFraction SATFloat32Bits `json:"day_fraction"`
	Day         uint16         `json:"day"`
	Year        uint8          `json:"year"`
}

func parseSATSection(i *bytesiter.Iterator, offsetSectionsEnd int, tableIDExtension uint16) (d *SAT, err error) {
	d = &SAT{
		SatelliteTableID: SatelliteTableID(tableIDExtension >> satTableCountBits),
		TableCount:       tableIDExtension & satTableCountMask,
	}
	if _, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	switch d.SatelliteTableID {
	case SatelliteTableIDPositionV2:
		d.SatellitePositionV2Info, err = parseSATSatellitePositionV2Info(i, offsetSectionsEnd)
	case SatelliteTableIDCellFragment:
		d.CellFragmentInfo, err = parseSATCellFragmentInfo(i, offsetSectionsEnd)
	case SatelliteTableIDTimeAssociation:
		d.TimeAssociationInfo, err = parseSATTimeAssociationInfo(i)
	case SatelliteTableIDBeamhoppingTimePlan:
		d.BeamhoppingTimePlanInfo, err = parseSATBeamhoppingTimePlanInfo(i, offsetSectionsEnd)
	case SatelliteTableIDPositionV3:
		d.SatellitePositionV3Info, err = parseSATSatellitePositionV3Info(i, offsetSectionsEnd)
	default:
		if n := offsetSectionsEnd - i.Offset(); n > 0 {
			if d.Reserved, err = i.NextBytes(n); err != nil {
				err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			}
		}
	}
	return
}

func (d *SAT) CalcSectionLength() (n int) {
	n = satReservedByteLen
	switch d.SatelliteTableID {
	case SatelliteTableIDPositionV2:
		for k := range d.SatellitePositionV2Info {
			n += d.SatellitePositionV2Info[k].calcLength()
		}
	case SatelliteTableIDCellFragment:
		for k := range d.CellFragmentInfo {
			n += d.CellFragmentInfo[k].calcLength()
		}
	case SatelliteTableIDTimeAssociation:
		n += satTimeAssociationLen
	case SatelliteTableIDBeamhoppingTimePlan:
		for k := range d.BeamhoppingTimePlanInfo {
			n += d.BeamhoppingTimePlanInfo[k].calcLength()
		}
	case SatelliteTableIDPositionV3:
		n += d.SatellitePositionV3Info.calcLength()
	default:
		n += len(d.Reserved)
	}
	return
}

func (d *SAT) tableIDExtension() uint16 {
	return uint16(d.SatelliteTableID)<<satTableCountBits | d.TableCount&satTableCountMask
}

func (d *SAT) appendSection(dst []byte) []byte {
	dst = append(dst, satReservedZeroFutureUse)
	switch d.SatelliteTableID {
	case SatelliteTableIDPositionV2:
		for k := range d.SatellitePositionV2Info {
			dst = d.SatellitePositionV2Info[k].appendTo(dst)
		}
	case SatelliteTableIDCellFragment:
		for k := range d.CellFragmentInfo {
			dst = d.CellFragmentInfo[k].appendTo(dst)
		}
	case SatelliteTableIDTimeAssociation:
		dst = d.TimeAssociationInfo.appendTo(dst)
	case SatelliteTableIDBeamhoppingTimePlan:
		for k := range d.BeamhoppingTimePlanInfo {
			dst = d.BeamhoppingTimePlanInfo[k].appendTo(dst)
		}
	case SatelliteTableIDPositionV3:
		dst = d.SatellitePositionV3Info.appendTo(dst)
	default:
		dst = append(dst, d.Reserved...)
	}
	return dst
}

func satNextBytes(i *bytesiter.Iterator, n int) (bs []byte, err error) {
	if bs, err = i.NextBytesNoCopy(n); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
	}
	return
}

func satFlag(on bool, bit byte) (b byte) {
	if on {
		b = bit
	}
	return
}

func satSignExtend(v uint32, bits int) (s int32) {
	shift := satSignExtendWidth - bits
	s = int32(v<<shift) >> shift
	return
}

func satUint24(bs []byte) uint32 {
	return uint32(bs[0])<<16 | uint32(binary.BigEndian.Uint16(bs[1:]))
}

func satAppendUint24(dst []byte, v uint32) []byte {
	return append(dst, byte(v>>16), byte(v>>8), byte(v))
}

func parseSATNCR(bs []byte) (n SATNCR) {
	v := uint64(binary.BigEndian.Uint16(bs))<<32 | uint64(binary.BigEndian.Uint32(bs[2:]))
	n = SATNCR{Base: v >> satNCRBaseShift, Ext: uint16(v & satNCRExtMask)}
	return
}

func (n SATNCR) appendTo(dst []byte) []byte {
	v := (n.Base&satNCRBaseMask)<<satNCRBaseShift | uint64(n.Ext&satNCRExtMask)
	return append(dst, byte(v>>40), byte(v>>32), byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

func parseSATYearDayTime(bs []byte) SATYearDayTime {
	return SATYearDayTime{
		Year:        bs[0],
		Day:         binary.BigEndian.Uint16(bs[satYearLen:]) & satDayMask,
		DayFraction: SATFloat32Bits(binary.BigEndian.Uint32(bs[satYearLen+satDayLen:])),
	}
}

func (t SATYearDayTime) appendTo(dst []byte) []byte {
	dst = append(dst, t.Year)
	dst = binary.BigEndian.AppendUint16(dst, t.Day&satDayMask)
	return binary.BigEndian.AppendUint32(dst, uint32(t.DayFraction))
}

func parseSATLoopCount(i *bytesiter.Iterator) (n int, err error) {
	var bs []byte
	if bs, err = satNextBytes(i, satLoopCountLen); err != nil {
		return
	}
	n = int(binary.BigEndian.Uint16(bs) & satLoopCountMask)
	return
}

const (
	satTLEElementCount            = 10
	satEpochYearLen               = 1
	satDayOfTheYearLen            = 2
	satPositionV2HeadLen          = satUint24Len + 1
	satPositionV2GeostationaryLen = 2 + 1
	satPositionV2TLELen           = satEpochYearLen + satDayOfTheYearLen + satTLEElementCount*satSPFLen
	satWestEastFlagBit            = 0x80
	satPositionSystemMask         = 0x01
)

type SATSatellitePositionV2 struct {
	SatelliteID                      uint32            `json:"satellite_id"`
	DayFraction                      SATFloat32Bits    `json:"day_fraction"`
	MeanMotionFirstDerivative        SATFloat32Bits    `json:"mean_motion_first_derivative"`
	MeanMotionSecondDerivative       SATFloat32Bits    `json:"mean_motion_second_derivative"`
	DragTerm                         SATFloat32Bits    `json:"drag_term"`
	Inclination                      SATFloat32Bits    `json:"inclination"`
	RightAscensionOfTheAscendingNode SATFloat32Bits    `json:"right_ascension_of_the_ascending_node"`
	Eccentricity                     SATFloat32Bits    `json:"eccentricity"`
	ArgumentOfPerigee                SATFloat32Bits    `json:"argument_of_perigee"`
	MeanAnomaly                      SATFloat32Bits    `json:"mean_anomaly"`
	MeanMotion                       SATFloat32Bits    `json:"mean_motion"`
	OrbitalPosition                  uint16            `json:"orbital_position"`
	DayOfTheYear                     uint16            `json:"day_of_the_year"`
	PositionSystem                   SATPositionSystem `json:"position_system"`
	EpochYear                        uint8             `json:"epoch_year"`
	WestEastFlag                     bool              `json:"west_east_flag"`
}

func (s *SATSatellitePositionV2) tleElements() [satTLEElementCount]*SATFloat32Bits {
	return [satTLEElementCount]*SATFloat32Bits{
		&s.DayFraction, &s.MeanMotionFirstDerivative, &s.MeanMotionSecondDerivative, &s.DragTerm, &s.Inclination,
		&s.RightAscensionOfTheAscendingNode, &s.Eccentricity, &s.ArgumentOfPerigee, &s.MeanAnomaly, &s.MeanMotion,
	}
}

func parseSATSatellitePositionV2Info(i *bytesiter.Iterator, offsetSectionsEnd int) (ss []SATSatellitePositionV2, err error) {
	var bs []byte
	for i.Offset() < offsetSectionsEnd {
		if bs, err = satNextBytes(i, satPositionV2HeadLen); err != nil {
			return
		}
		s := SATSatellitePositionV2{SatelliteID: satUint24(bs), PositionSystem: SATPositionSystem(bs[satUint24Len] & satPositionSystemMask)}
		if s.PositionSystem == SATPositionSystemEarthOrbiting {
			if bs, err = satNextBytes(i, satPositionV2TLELen); err != nil {
				return
			}
			s.EpochYear = bs[0]
			s.DayOfTheYear = binary.BigEndian.Uint16(bs[satEpochYearLen:])
			bs = bs[satEpochYearLen+satDayOfTheYearLen:]
			for _, e := range s.tleElements() {
				*e = SATFloat32Bits(binary.BigEndian.Uint32(bs))
				bs = bs[satSPFLen:]
			}
		} else {
			if bs, err = satNextBytes(i, satPositionV2GeostationaryLen); err != nil {
				return
			}
			s.OrbitalPosition = binary.BigEndian.Uint16(bs)
			s.WestEastFlag = bs[2]&satWestEastFlagBit > 0
		}
		ss = append(ss, s)
	}
	return
}

func (s *SATSatellitePositionV2) calcLength() (n int) {
	n = satPositionV2HeadLen + satPositionV2GeostationaryLen
	if s.PositionSystem == SATPositionSystemEarthOrbiting {
		n = satPositionV2HeadLen + satPositionV2TLELen
	}
	return
}

func (s *SATSatellitePositionV2) appendTo(dst []byte) []byte {
	dst = satAppendUint24(dst, s.SatelliteID)
	if s.PositionSystem == SATPositionSystemEarthOrbiting {
		dst = append(dst, byte(SATPositionSystemEarthOrbiting), s.EpochYear)
		dst = binary.BigEndian.AppendUint16(dst, s.DayOfTheYear)
		for _, e := range s.tleElements() {
			dst = binary.BigEndian.AppendUint32(dst, uint32(*e))
		}
		return dst
	}
	dst = append(dst, byte(SATPositionSystemGeostationary))
	dst = binary.BigEndian.AppendUint16(dst, s.OrbitalPosition)
	return append(dst, satFlag(s.WestEastFlag, satWestEastFlagBit))
}

const (
	satCellFragmentIDLen     = 4
	satCellFragmentFlagsLen  = 1
	satCellFragmentCenterLen = satCellFragmentFlagsLen + 2 + satUint24Len + satUint24Len
	satLoopCountLen          = 2
	satLoopCountBits         = 10
	satLoopCountMask         = 1<<satLoopCountBits - 1
	satDeliverySystemIDLen   = 4
	satDeliverySystemNCRLen  = satDeliverySystemIDLen + satNCRLen
	satCenterLatitudeBits    = 18
	satCenterLatitudeMask    = 1<<satCenterLatitudeBits - 1
	satCenterLatitudeLowBits = 16
	satCenterLongitudeBits   = 19
	satCenterLongitudeMask   = 1<<satCenterLongitudeBits - 1
	satFirstOccurrenceBit    = 0x80
	satLastOccurrenceBit     = 0x40
)

type SATCellFragment struct {
	DeliverySystemIDs          []uint32                       `json:"_delivery_system_ids"`
	NewDeliverySystems         []SATNewDeliverySystem         `json:"_new_delivery_systems"`
	ObsolescentDeliverySystems []SATObsolescentDeliverySystem `json:"_obsolescent_delivery_systems"`
	CellFragmentID             uint32                         `json:"cell_fragment_id"`
	MaxDistance                uint32                         `json:"max_distance"`
	CenterLatitude             int32                          `json:"center_latitude"`
	CenterLongitude            int32                          `json:"center_longitude"`
	FirstOccurrence            bool                           `json:"first_occurrence"`
	LastOccurrence             bool                           `json:"last_occurrence"`
}

type SATNewDeliverySystem struct {
	TimeOfApplication   SATNCR `json:"time_of_application"`
	NewDeliverySystemID uint32 `json:"new_delivery_system_id"`
}

type SATObsolescentDeliverySystem struct {
	TimeOfObsolescence          SATNCR `json:"time_of_obsolescence"`
	ObsolescentDeliverySystemID uint32 `json:"obsolescent_delivery_system_id"`
}

func parseSATCellFragmentInfo(i *bytesiter.Iterator, offsetSectionsEnd int) (cs []SATCellFragment, err error) {
	var bs []byte
	for i.Offset() < offsetSectionsEnd {
		if bs, err = satNextBytes(i, satCellFragmentIDLen+satCellFragmentFlagsLen); err != nil {
			return
		}
		flags := bs[satCellFragmentIDLen]
		c := SATCellFragment{
			CellFragmentID:  binary.BigEndian.Uint32(bs),
			FirstOccurrence: flags&satFirstOccurrenceBit > 0,
			LastOccurrence:  flags&satLastOccurrenceBit > 0,
		}
		countHi, rest := flags, satLoopCountLen-satCellFragmentFlagsLen
		if c.FirstOccurrence {
			rest += satCellFragmentCenterLen
		}
		if bs, err = satNextBytes(i, rest); err != nil {
			return
		}
		if c.FirstOccurrence {
			c.CenterLatitude = satSignExtend(uint32(flags)<<satCenterLatitudeLowBits|uint32(binary.BigEndian.Uint16(bs)), satCenterLatitudeBits)
			bs = bs[2:]
			c.CenterLongitude = satSignExtend(satUint24(bs), satCenterLongitudeBits)
			bs = bs[satUint24Len:]
			c.MaxDistance = satUint24(bs)
			bs = bs[satUint24Len:]
			countHi, bs = bs[0], bs[1:]
		}
		count := int(uint16(countHi)<<8|uint16(bs[0])) & satLoopCountMask
		for range count {
			if bs, err = satNextBytes(i, satDeliverySystemIDLen); err != nil {
				return
			}
			c.DeliverySystemIDs = append(c.DeliverySystemIDs, binary.BigEndian.Uint32(bs))
		}
		if count, err = parseSATLoopCount(i); err != nil {
			return
		}
		for range count {
			if bs, err = satNextBytes(i, satDeliverySystemNCRLen); err != nil {
				return
			}
			c.NewDeliverySystems = append(c.NewDeliverySystems, SATNewDeliverySystem{
				NewDeliverySystemID: binary.BigEndian.Uint32(bs),
				TimeOfApplication:   parseSATNCR(bs[satDeliverySystemIDLen:]),
			})
		}
		if count, err = parseSATLoopCount(i); err != nil {
			return
		}
		for range count {
			if bs, err = satNextBytes(i, satDeliverySystemNCRLen); err != nil {
				return
			}
			c.ObsolescentDeliverySystems = append(c.ObsolescentDeliverySystems, SATObsolescentDeliverySystem{
				ObsolescentDeliverySystemID: binary.BigEndian.Uint32(bs),
				TimeOfObsolescence:          parseSATNCR(bs[satDeliverySystemIDLen:]),
			})
		}
		cs = append(cs, c)
	}
	return
}

func (c *SATCellFragment) calcLength() (n int) {
	n = satCellFragmentIDLen + satLoopCountLen
	if c.FirstOccurrence {
		n += satCellFragmentCenterLen
	}
	n += len(c.DeliverySystemIDs) * satDeliverySystemIDLen
	n += satLoopCountLen + len(c.NewDeliverySystems)*satDeliverySystemNCRLen
	n += satLoopCountLen + len(c.ObsolescentDeliverySystems)*satDeliverySystemNCRLen
	return
}

func (c *SATCellFragment) appendTo(dst []byte) []byte {
	dst = binary.BigEndian.AppendUint32(dst, c.CellFragmentID)
	flags := satFlag(c.FirstOccurrence, satFirstOccurrenceBit) | satFlag(c.LastOccurrence, satLastOccurrenceBit)
	count := uint16(len(c.DeliverySystemIDs)) & satLoopCountMask
	if c.FirstOccurrence {
		lat := uint32(c.CenterLatitude) & satCenterLatitudeMask
		dst = append(dst, flags|byte(lat>>satCenterLatitudeLowBits), byte(lat>>8), byte(lat))
		dst = satAppendUint24(dst, uint32(c.CenterLongitude)&satCenterLongitudeMask)
		dst = satAppendUint24(dst, c.MaxDistance)
		dst = binary.BigEndian.AppendUint16(dst, count)
	} else {
		dst = append(dst, flags|byte(count>>8), byte(count))
	}
	for _, id := range c.DeliverySystemIDs {
		dst = binary.BigEndian.AppendUint32(dst, id)
	}
	dst = binary.BigEndian.AppendUint16(dst, uint16(len(c.NewDeliverySystems))&satLoopCountMask)
	for k := range c.NewDeliverySystems {
		dst = binary.BigEndian.AppendUint32(dst, c.NewDeliverySystems[k].NewDeliverySystemID)
		dst = c.NewDeliverySystems[k].TimeOfApplication.appendTo(dst)
	}
	dst = binary.BigEndian.AppendUint16(dst, uint16(len(c.ObsolescentDeliverySystems))&satLoopCountMask)
	for k := range c.ObsolescentDeliverySystems {
		dst = binary.BigEndian.AppendUint32(dst, c.ObsolescentDeliverySystems[k].ObsolescentDeliverySystemID)
		dst = c.ObsolescentDeliverySystems[k].TimeOfObsolescence.appendTo(dst)
	}
	return dst
}

const (
	satAssociationTimestampSecondsLen = 8
	satTimeAssociationLen             = 1 + satNCRLen + satAssociationTimestampSecondsLen + 4
	satAssociationTypeShift           = 4
	satLeap59Bit                      = 0x08
	satLeap61Bit                      = 0x04
	satPastLeap59Bit                  = 0x02
	satPastLeap61Bit                  = 0x01
)

type SATTimeAssociation struct {
	AssociationTimestampSeconds     uint64             `json:"association_timestamp_seconds"`
	NCR                             SATNCR             `json:"ncr"`
	AssociationTimestampNanoseconds uint32             `json:"association_timestamp_nanoseconds"`
	AssociationType                 SATAssociationType `json:"association_type"`
	Leap59                          bool               `json:"leap59"`
	Leap61                          bool               `json:"leap61"`
	PastLeap59                      bool               `json:"pastleap59"`
	PastLeap61                      bool               `json:"pastleap61"`
}

func parseSATTimeAssociationInfo(i *bytesiter.Iterator) (t SATTimeAssociation, err error) {
	var bs []byte
	if bs, err = satNextBytes(i, satTimeAssociationLen); err != nil {
		return
	}
	t.AssociationType = SATAssociationType(bs[0] >> satAssociationTypeShift)
	if t.AssociationType == SATAssociationTypeUTCLeapSeconds {
		t.Leap59 = bs[0]&satLeap59Bit > 0
		t.Leap61 = bs[0]&satLeap61Bit > 0
		t.PastLeap59 = bs[0]&satPastLeap59Bit > 0
		t.PastLeap61 = bs[0]&satPastLeap61Bit > 0
	}
	bs = bs[1:]
	t.NCR = parseSATNCR(bs)
	bs = bs[satNCRLen:]
	t.AssociationTimestampSeconds = binary.BigEndian.Uint64(bs)
	t.AssociationTimestampNanoseconds = binary.BigEndian.Uint32(bs[satAssociationTimestampSecondsLen:])
	return
}

func (t *SATTimeAssociation) appendTo(dst []byte) []byte {
	b := byte(t.AssociationType) << satAssociationTypeShift
	if t.AssociationType == SATAssociationTypeUTCLeapSeconds {
		b |= satFlag(t.Leap59, satLeap59Bit) | satFlag(t.Leap61, satLeap61Bit) |
			satFlag(t.PastLeap59, satPastLeap59Bit) | satFlag(t.PastLeap61, satPastLeap61Bit)
	}
	dst = append(dst, b)
	dst = t.NCR.appendTo(dst)
	dst = binary.BigEndian.AppendUint64(dst, t.AssociationTimestampSeconds)
	return binary.BigEndian.AppendUint32(dst, t.AssociationTimestampNanoseconds)
}

const (
	satTimePlanIDLen         = 4
	satTimePlanLengthLen     = 2
	satTimePlanLengthMask    = 1<<12 - 1
	satTimePlanModeLen       = 1
	satTimePlanModeMask      = 0x03
	satTimePlanHeadLen       = satTimePlanIDLen + satTimePlanLengthLen + satTimePlanModeLen + 2*satNCRLen
	satTimePlanDwellLen      = 2 * satNCRLen
	satTimePlanBitMapHeadLen = 2 + 2
	satTimePlanSlotMask      = 1<<15 - 1
	satTimePlanGridLen       = 4 * satNCRLen
)

type SATBeamhoppingTimePlan struct {
	SlotTransmissionOn    []bool          `json:"_slot_transmission_on,omitempty"`
	Reserved              []byte          `json:"_reserved,omitempty"`
	TimeOfApplication     SATNCR          `json:"time_of_application"`
	CycleDuration         SATNCR          `json:"cycle_duration"`
	DwellDuration         SATNCR          `json:"dwell_duration"`
	OnTime                SATNCR          `json:"on_time"`
	GridSize              SATNCR          `json:"grid_size"`
	RevisitDuration       SATNCR          `json:"revisit_duration"`
	SleepTime             SATNCR          `json:"sleep_time"`
	SleepDuration         SATNCR          `json:"sleep_duration"`
	BeamhoppingTimePlanID uint32          `json:"beamhopping_time_plan_id"`
	CurrentSlot           uint16          `json:"current_slot"`
	TimePlanMode          SATTimePlanMode `json:"time_plan_mode"`
}

func satBitMapLen(bits int) int {
	return (bits + bitsPerByte - 1) / bitsPerByte
}

func parseSATBeamhoppingTimePlanInfo(i *bytesiter.Iterator, offsetSectionsEnd int) (ps []SATBeamhoppingTimePlan, err error) {
	var bs []byte
	for i.Offset() < offsetSectionsEnd {
		start := i.Offset()
		if bs, err = satNextBytes(i, satTimePlanHeadLen); err != nil {
			return
		}
		p := SATBeamhoppingTimePlan{BeamhoppingTimePlanID: binary.BigEndian.Uint32(bs)}
		bs = bs[satTimePlanIDLen:]
		end := start + int(binary.BigEndian.Uint16(bs)&satTimePlanLengthMask)
		bs = bs[satTimePlanLengthLen:]
		p.TimePlanMode = SATTimePlanMode(bs[0] & satTimePlanModeMask)
		bs = bs[satTimePlanModeLen:]
		p.TimeOfApplication = parseSATNCR(bs)
		p.CycleDuration = parseSATNCR(bs[satNCRLen:])
		switch p.TimePlanMode {
		case SATTimePlanModeDwell:
			if bs, err = satNextBytes(i, satTimePlanDwellLen); err != nil {
				return
			}
			p.DwellDuration = parseSATNCR(bs)
			p.OnTime = parseSATNCR(bs[satNCRLen:])
		case SATTimePlanModeBitMap:
			if bs, err = satNextBytes(i, satTimePlanBitMapHeadLen); err != nil {
				return
			}
			bits := int(binary.BigEndian.Uint16(bs) & satTimePlanSlotMask)
			p.CurrentSlot = binary.BigEndian.Uint16(bs[2:]) & satTimePlanSlotMask
			if bs, err = satNextBytes(i, satBitMapLen(bits)); err != nil {
				return
			}
			p.SlotTransmissionOn = make([]bool, bits)
			for j := range bits {
				p.SlotTransmissionOn[j] = bs[j/bitsPerByte]&(satByteMSBMask>>(j%bitsPerByte)) > 0
			}
		case SATTimePlanModeGrid:
			if bs, err = satNextBytes(i, satTimePlanGridLen); err != nil {
				return
			}
			p.GridSize = parseSATNCR(bs)
			p.RevisitDuration = parseSATNCR(bs[satNCRLen:])
			p.SleepTime = parseSATNCR(bs[2*satNCRLen:])
			p.SleepDuration = parseSATNCR(bs[3*satNCRLen:])
		}
		if i.Offset() > end {
			err = fmt.Errorf("astits: beamhopping_time_plan_length %d ends before the fields of time_plan_mode %d: %w", end-start, p.TimePlanMode, ErrSATTimePlanLength)
			return
		}
		if n := end - i.Offset(); n > 0 {
			if p.Reserved, err = i.NextBytes(n); err != nil {
				err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
				return
			}
		}
		ps = append(ps, p)
	}
	return
}

func (p *SATBeamhoppingTimePlan) calcLength() (n int) {
	n = satTimePlanHeadLen + len(p.Reserved)
	switch p.TimePlanMode {
	case SATTimePlanModeDwell:
		n += satTimePlanDwellLen
	case SATTimePlanModeBitMap:
		n += satTimePlanBitMapHeadLen + satBitMapLen(len(p.SlotTransmissionOn))
	case SATTimePlanModeGrid:
		n += satTimePlanGridLen
	}
	return
}

func (p *SATBeamhoppingTimePlan) appendTo(dst []byte) []byte {
	dst = binary.BigEndian.AppendUint32(dst, p.BeamhoppingTimePlanID)
	dst = binary.BigEndian.AppendUint16(dst, uint16(p.calcLength())&satTimePlanLengthMask)
	dst = append(dst, byte(p.TimePlanMode)&satTimePlanModeMask)
	dst = p.TimeOfApplication.appendTo(dst)
	dst = p.CycleDuration.appendTo(dst)
	switch p.TimePlanMode {
	case SATTimePlanModeDwell:
		dst = p.DwellDuration.appendTo(dst)
		dst = p.OnTime.appendTo(dst)
	case SATTimePlanModeBitMap:
		dst = binary.BigEndian.AppendUint16(dst, uint16(len(p.SlotTransmissionOn))&satTimePlanSlotMask)
		dst = binary.BigEndian.AppendUint16(dst, p.CurrentSlot&satTimePlanSlotMask)
		for j, on := range p.SlotTransmissionOn {
			if j%bitsPerByte == 0 {
				dst = append(dst, 0)
			}
			if on {
				dst[len(dst)-1] |= satByteMSBMask >> (j % bitsPerByte)
			}
		}
	case SATTimePlanModeGrid:
		dst = p.GridSize.appendTo(dst)
		dst = p.RevisitDuration.appendTo(dst)
		dst = p.SleepTime.appendTo(dst)
		dst = p.SleepDuration.appendTo(dst)
	}
	return append(dst, p.Reserved...)
}

const (
	satPositionV3HeadLen      = 1 + satYearDayTimeLen
	satOEMVersionMajorShift   = 4
	satOEMVersionMinorMask    = 0x0f
	satEphemerisFlagsLen      = 1
	satMetadataFlagBit        = 0x10
	satUsableStartTimeFlagBit = 0x08
	satUsableStopTimeFlagBit  = 0x04
	satEphemerisAccelFlagBit  = 0x02
	satCovarianceFlagBit      = 0x01
	satMetadataHeadLen        = 2*satYearDayTimeLen + 1
	satInterpolationFlagBit   = 0x40
	satInterpolationTypeShift = 3
	satInterpolationMask      = 0x07
	satEphemerisDataCountLen  = 2
	satStateVectorCount       = 6
	satStateVectorLen         = satStateVectorCount * satSPFLen
	satEphemerisRecordLen     = satYearDayTimeLen + satStateVectorLen
	satEphemerisAccelCount    = 3
	satEphemerisAccelLen      = satEphemerisAccelCount * satSPFLen
	satCovarianceLen          = satYearDayTimeLen + SATCovarianceElementCount*satSPFLen
)

type SATSatellitePositionV3 struct {
	Satellites      []SATSatelliteEphemeris `json:"_satellites"`
	CreationDate    SATYearDayTime          `json:"creation_date"`
	OEMVersionMajor uint8                   `json:"oem_version_major"`
	OEMVersionMinor uint8                   `json:"oem_version_minor"`
}

type SATSatelliteEphemeris struct {
	EphemerisData       []SATEphemerisRecord `json:"_ephemeris_data"`
	Metadata            SATEphemerisMetadata `json:"_metadata"`
	Covariance          SATCovariance        `json:"_covariance"`
	SatelliteID         uint32               `json:"satellite_id"`
	MetadataFlag        bool                 `json:"metadata_flag"`
	UsableStartTimeFlag bool                 `json:"usable_start_time_flag"`
	UsableStopTimeFlag  bool                 `json:"usable_stop_time_flag"`
	EphemerisAccelFlag  bool                 `json:"ephemeris_accel_flag"`
	CovarianceFlag      bool                 `json:"covariance_flag"`
}

type SATEphemerisMetadata struct {
	TotalStartTime      SATYearDayTime       `json:"total_start_time"`
	TotalStopTime       SATYearDayTime       `json:"total_stop_time"`
	UsableStartTime     SATYearDayTime       `json:"usable_start_time"`
	UsableStopTime      SATYearDayTime       `json:"usable_stop_time"`
	InterpolationType   SATInterpolationType `json:"interpolation_type"`
	InterpolationDegree uint8                `json:"interpolation_degree"`
	InterpolationFlag   bool                 `json:"interpolation_flag"`
}

type SATEphemerisRecord struct {
	Epoch          SATYearDayTime `json:"epoch"`
	EphemerisX     SATFloat32Bits `json:"ephemeris_x"`
	EphemerisY     SATFloat32Bits `json:"ephemeris_y"`
	EphemerisZ     SATFloat32Bits `json:"ephemeris_z"`
	EphemerisXDot  SATFloat32Bits `json:"ephemeris_x_dot"`
	EphemerisYDot  SATFloat32Bits `json:"ephemeris_y_dot"`
	EphemerisZDot  SATFloat32Bits `json:"ephemeris_z_dot"`
	EphemerisXDdot SATFloat32Bits `json:"ephemeris_x_ddot"`
	EphemerisYDdot SATFloat32Bits `json:"ephemeris_y_ddot"`
	EphemerisZDdot SATFloat32Bits `json:"ephemeris_z_ddot"`
}

// CovarianceElements run from [1,1] to [6,6] of the lower triangle, row by row (EN 300 468 §5.2.11.6).
type SATCovariance struct {
	CovarianceElements [SATCovarianceElementCount]SATFloat32Bits `json:"covariance_elements"`
	CovarianceEpoch    SATYearDayTime                            `json:"covariance_epoch"`
}

func parseSATSatellitePositionV3Info(i *bytesiter.Iterator, offsetSectionsEnd int) (d SATSatellitePositionV3, err error) {
	var bs []byte
	if bs, err = satNextBytes(i, satPositionV3HeadLen); err != nil {
		return
	}
	d.OEMVersionMajor = bs[0] >> satOEMVersionMajorShift
	d.OEMVersionMinor = bs[0] & satOEMVersionMinorMask
	d.CreationDate = parseSATYearDayTime(bs[1:])
	for i.Offset() < offsetSectionsEnd {
		var s SATSatelliteEphemeris
		if s, err = parseSATSatelliteEphemeris(i); err != nil {
			return
		}
		d.Satellites = append(d.Satellites, s)
	}
	return
}

func (d *SATSatellitePositionV3) calcLength() (n int) {
	n = satPositionV3HeadLen
	for k := range d.Satellites {
		n += d.Satellites[k].calcLength()
	}
	return
}

func (d *SATSatellitePositionV3) appendTo(dst []byte) []byte {
	dst = append(dst, d.OEMVersionMajor<<satOEMVersionMajorShift|d.OEMVersionMinor&satOEMVersionMinorMask)
	dst = d.CreationDate.appendTo(dst)
	for k := range d.Satellites {
		dst = d.Satellites[k].appendTo(dst)
	}
	return dst
}

func parseSATSatelliteEphemeris(i *bytesiter.Iterator) (s SATSatelliteEphemeris, err error) {
	var bs []byte
	if bs, err = satNextBytes(i, satUint24Len+satEphemerisFlagsLen); err != nil {
		return
	}
	s.SatelliteID = satUint24(bs)
	flags := bs[satUint24Len]
	s.MetadataFlag = flags&satMetadataFlagBit > 0
	s.UsableStartTimeFlag = flags&satUsableStartTimeFlagBit > 0
	s.UsableStopTimeFlag = flags&satUsableStopTimeFlagBit > 0
	s.EphemerisAccelFlag = flags&satEphemerisAccelFlagBit > 0
	s.CovarianceFlag = flags&satCovarianceFlagBit > 0
	if s.MetadataFlag {
		if s.Metadata, err = parseSATEphemerisMetadata(i, s.UsableStartTimeFlag, s.UsableStopTimeFlag); err != nil {
			return
		}
	}
	if bs, err = satNextBytes(i, satEphemerisDataCountLen); err != nil {
		return
	}
	count := binary.BigEndian.Uint16(bs)
	recordLen := satEphemerisRecordLen
	if s.EphemerisAccelFlag {
		recordLen += satEphemerisAccelLen
	}
	for range count {
		if bs, err = satNextBytes(i, recordLen); err != nil {
			return
		}
		s.EphemerisData = append(s.EphemerisData, parseSATEphemerisRecord(bs, s.EphemerisAccelFlag))
	}
	if s.CovarianceFlag {
		if bs, err = satNextBytes(i, satCovarianceLen); err != nil {
			return
		}
		s.Covariance.CovarianceEpoch = parseSATYearDayTime(bs)
		bs = bs[satYearDayTimeLen:]
		for k := range s.Covariance.CovarianceElements {
			s.Covariance.CovarianceElements[k] = SATFloat32Bits(binary.BigEndian.Uint32(bs[k*satSPFLen:]))
		}
	}
	return
}

func (s *SATSatelliteEphemeris) calcLength() (n int) {
	n = satUint24Len + satEphemerisFlagsLen + satEphemerisDataCountLen
	if s.MetadataFlag {
		n += s.Metadata.calcLength(s.UsableStartTimeFlag, s.UsableStopTimeFlag)
	}
	recordLen := satEphemerisRecordLen
	if s.EphemerisAccelFlag {
		recordLen += satEphemerisAccelLen
	}
	n += len(s.EphemerisData) * recordLen
	if s.CovarianceFlag {
		n += satCovarianceLen
	}
	return
}

func (s *SATSatelliteEphemeris) appendTo(dst []byte) []byte {
	dst = satAppendUint24(dst, s.SatelliteID)
	dst = append(dst, satFlag(s.MetadataFlag, satMetadataFlagBit)|
		satFlag(s.UsableStartTimeFlag, satUsableStartTimeFlagBit)|
		satFlag(s.UsableStopTimeFlag, satUsableStopTimeFlagBit)|
		satFlag(s.EphemerisAccelFlag, satEphemerisAccelFlagBit)|
		satFlag(s.CovarianceFlag, satCovarianceFlagBit))
	if s.MetadataFlag {
		dst = s.Metadata.appendTo(dst, s.UsableStartTimeFlag, s.UsableStopTimeFlag)
	}
	dst = binary.BigEndian.AppendUint16(dst, uint16(len(s.EphemerisData)))
	for k := range s.EphemerisData {
		dst = s.EphemerisData[k].appendTo(dst, s.EphemerisAccelFlag)
	}
	if s.CovarianceFlag {
		dst = s.Covariance.CovarianceEpoch.appendTo(dst)
		for _, e := range s.Covariance.CovarianceElements {
			dst = binary.BigEndian.AppendUint32(dst, uint32(e))
		}
	}
	return dst
}

func parseSATEphemerisMetadata(i *bytesiter.Iterator, usableStart, usableStop bool) (m SATEphemerisMetadata, err error) {
	var bs []byte
	if bs, err = satNextBytes(i, satMetadataHeadLen); err != nil {
		return
	}
	m.TotalStartTime = parseSATYearDayTime(bs)
	bs = bs[satYearDayTimeLen:]
	m.TotalStopTime = parseSATYearDayTime(bs)
	bs = bs[satYearDayTimeLen:]
	m.InterpolationFlag = bs[0]&satInterpolationFlagBit > 0
	m.InterpolationType = SATInterpolationType(bs[0] >> satInterpolationTypeShift & satInterpolationMask)
	m.InterpolationDegree = bs[0] & satInterpolationMask
	if usableStart {
		if bs, err = satNextBytes(i, satYearDayTimeLen); err != nil {
			return
		}
		m.UsableStartTime = parseSATYearDayTime(bs)
	}
	if usableStop {
		if bs, err = satNextBytes(i, satYearDayTimeLen); err != nil {
			return
		}
		m.UsableStopTime = parseSATYearDayTime(bs)
	}
	return
}

func (m *SATEphemerisMetadata) calcLength(usableStart, usableStop bool) (n int) {
	n = satMetadataHeadLen
	if usableStart {
		n += satYearDayTimeLen
	}
	if usableStop {
		n += satYearDayTimeLen
	}
	return
}

func (m *SATEphemerisMetadata) appendTo(dst []byte, usableStart, usableStop bool) []byte {
	dst = m.TotalStartTime.appendTo(dst)
	dst = m.TotalStopTime.appendTo(dst)
	dst = append(dst, satFlag(m.InterpolationFlag, satInterpolationFlagBit)|
		(byte(m.InterpolationType)&satInterpolationMask)<<satInterpolationTypeShift|
		m.InterpolationDegree&satInterpolationMask)
	if usableStart {
		dst = m.UsableStartTime.appendTo(dst)
	}
	if usableStop {
		dst = m.UsableStopTime.appendTo(dst)
	}
	return dst
}

func (r *SATEphemerisRecord) stateVector() [satStateVectorCount]*SATFloat32Bits {
	return [satStateVectorCount]*SATFloat32Bits{&r.EphemerisX, &r.EphemerisY, &r.EphemerisZ, &r.EphemerisXDot, &r.EphemerisYDot, &r.EphemerisZDot}
}

func (r *SATEphemerisRecord) acceleration() [satEphemerisAccelCount]*SATFloat32Bits {
	return [satEphemerisAccelCount]*SATFloat32Bits{&r.EphemerisXDdot, &r.EphemerisYDdot, &r.EphemerisZDdot}
}

func parseSATEphemerisRecord(bs []byte, accel bool) (r SATEphemerisRecord) {
	r.Epoch = parseSATYearDayTime(bs)
	bs = bs[satYearDayTimeLen:]
	for _, f := range r.stateVector() {
		*f = SATFloat32Bits(binary.BigEndian.Uint32(bs))
		bs = bs[satSPFLen:]
	}
	if accel {
		for _, f := range r.acceleration() {
			*f = SATFloat32Bits(binary.BigEndian.Uint32(bs))
			bs = bs[satSPFLen:]
		}
	}
	return
}

func (r *SATEphemerisRecord) appendTo(dst []byte, accel bool) []byte {
	dst = r.Epoch.appendTo(dst)
	for _, f := range r.stateVector() {
		dst = binary.BigEndian.AppendUint32(dst, uint32(*f))
	}
	if accel {
		for _, f := range r.acceleration() {
			dst = binary.BigEndian.AppendUint32(dst, uint32(*f))
		}
	}
	return dst
}
