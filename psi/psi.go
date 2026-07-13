package psi

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/errclass"
	"github.com/k-danil/go-astits/v2/internal/util"
	"github.com/k-danil/go-astits/v2/ts"
)

// PSI table IDs
const (
	TableTypeBAT      = "BAT"
	TableTypeCAT      = "CAT"
	TableTypeDIT      = "DIT"
	TableTypeEIT      = "EIT"
	TableTypeISO14496 = "ISO14496"
	TableTypeMetadata = "Metadata"
	TableTypeNIT      = "NIT"
	TableTypeNull     = "Null"
	TableTypePAT      = "PAT"
	TableTypePMT      = "PMT"
	TableTypeRST      = "RST"
	TableTypeSDT      = "SDT"
	TableTypeSIT      = "SIT"
	TableTypeST       = "ST"
	TableTypeTDT      = "TDT"
	TableTypeTOT      = "TOT"
	TableTypeTSDT     = "TSDT"
	TableTypeUnknown  = "Unknown"
)

// ErrCRC32Mismatch reports a section whose CRC32 does not match its content.
var ErrCRC32Mismatch = errclass.New("astits: CRC32 mismatch", ts.ErrInvalidData)

// ErrTableNotImplemented reports a table type whose serialization is not implemented.
var ErrTableNotImplemented = errors.New("astits: table serialization is not implemented")

// ErrSectionOverflow reports table data that does not fit the 1021-byte section
// limit; only PAT may span multiple sections, a PMT must fit one by spec.
var ErrSectionOverflow = errors.New("astits: section data does not fit a single section")

// maxSectionLength bounds the section_length field (12 bits, capped by spec).
const maxSectionLength = 1021

// TableID identifies a PSI table (PAT, PMT, EIT, NIT, SDT, TOT, ...).
type TableID uint8

const (
	TableIDPAT  TableID = 0x00
	TableIDCAT  TableID = 0x01
	TableIDPMT  TableID = 0x02
	TableIDTSDT TableID = 0x03

	TableIDISO14496Scene  TableID = 0x04
	TableIDISO14496Object TableID = 0x05
	TableIDMetadata       TableID = 0x06
	TableIDISO14496       TableID = 0x08

	TableIDNITVariant1 TableID = 0x40
	TableIDNITVariant2 TableID = 0x41
	TableIDSDTVariant1 TableID = 0x42
	TableIDSDTVariant2 TableID = 0x46

	TableIDBAT TableID = 0x4a

	TableIDEITStart TableID = 0x4e
	TableIDEITEnd   TableID = 0x6f

	TableIDTDT TableID = 0x70
	TableIDRST TableID = 0x71
	TableIDST  TableID = 0x72
	TableIDTOT TableID = 0x73

	TableIDDIT TableID = 0x7e
	TableIDSIT TableID = 0x7f

	TableIDNull TableID = 0xff
)

const (
	tableIDEITOtherPresentFollowing TableID = 0x4f
	tableIDEITActualScheduleStart   TableID = 0x50
	tableIDEITActualScheduleEnd     TableID = 0x5f
	tableIDEITOtherScheduleStart    TableID = 0x60
)

var tableIDNames = map[TableID]string{
	TableIDPAT:                      "program_association_section",
	TableIDCAT:                      "conditional_access_section",
	TableIDPMT:                      "TS_program_map_section",
	TableIDTSDT:                     "TS_description_section",
	TableIDISO14496Scene:            "ISO_IEC_14496_scene_description_section",
	TableIDISO14496Object:           "ISO_IEC_14496_object_descriptor_section",
	TableIDMetadata:                 "Metadata_section",
	TableIDISO14496:                 "ISO_IEC_14496_section",
	TableIDNITVariant1:              "network_information_section - actual_network",
	TableIDNITVariant2:              "network_information_section - other_network",
	TableIDSDTVariant1:              "service_description_section - actual_transport_stream",
	TableIDSDTVariant2:              "service_description_section - other_transport_stream",
	TableIDBAT:                      "bouquet_association_section",
	TableIDEITStart:                 "event_information_section - actual_transport_stream, present/following",
	tableIDEITOtherPresentFollowing: "event_information_section - other_transport_stream, present/following",
	TableIDTDT:                      "time_date_section",
	TableIDRST:                      "running_status_section",
	TableIDST:                       "stuffing_section",
	TableIDTOT:                      "time_offset_section",
	TableIDDIT:                      "discontinuity_information_section",
	TableIDSIT:                      "selection_information_section",
	TableIDNull:                     "forbidden",
}

func (t TableID) String() (s string) {
	var ok bool
	if s, ok = tableIDNames[t]; ok {
		return
	}
	switch {
	case t >= tableIDEITActualScheduleStart && t <= tableIDEITActualScheduleEnd:
		s = fmt.Sprintf("event_information_section - actual_transport_stream, schedule (0x%02x)", uint8(t))
	case t >= tableIDEITOtherScheduleStart && t <= TableIDEITEnd:
		s = fmt.Sprintf("event_information_section - other_transport_stream, schedule (0x%02x)", uint8(t))
	default:
		s = fmt.Sprintf("0x%02x", uint8(t))
	}
	return
}

func (t TableID) MarshalJSON() (b []byte, err error) {
	return json.Marshal(t.String())
}

func (t *TableID) UnmarshalJSON(b []byte) (err error) {
	*t, err = util.UnmarshalEnum(b, tableIDNames)
	return
}

// Data represents a PSI data
// https://en.wikipedia.org/wiki/Program-specific_information
type Data struct {
	PointerField int       `json:"pointer_field"` // Present at the start of the TS packet payload signaled by the payload_unit_start_indicator bit in the TS header. Used to set packet alignment bytes or content before the start of tabled payload data.
	Sections     []Section `json:"_sections"`
}

// Section represents a PSI section
type Section struct {
	Syntax *SectionSyntax `json:"_syntax"`
	CRC32  uint32         `json:"_crc32"` // A checksum of the entire table excluding the pointer field, pointer filler bytes and the trailing CRC32.
	Header SectionHeader  `json:"_header"`
}

// SectionHeader represents a PSI section header
type SectionHeader struct {
	SectionLength          uint16  `json:"section_length"`           // The number of bytes that follow for the syntax section (with CRC value) and/or table data. These bytes must not exceed a value of 1021.
	TableID                TableID `json:"table_id"`                 // Table Identifier, that defines the structure of the syntax section and other contained data. As an exception, if this is the byte that immediately follow previous table section and is set to 0xFF, then it indicates that the repeat of table section end here and the rest of TS data payload shall be stuffed with 0xFF. Consequently the value 0xFF shall not be used for the Table Identifier.
	SectionSyntaxIndicator bool    `json:"section_syntax_indicator"` // A flag that indicates if the syntax section follows the section length. The PAT, PMT, and CAT all set this to 1.
	PrivateBit             bool    `json:"private_indicator"`        // The PAT, PMT, and CAT all set this to 0. Other tables set this to 1.
	RandomAccessIndicator  bool    `json:"random_access_indicator"`  // metadata_section only; unused for every other table
	DecoderConfigFlag      bool    `json:"decoder_config_flag"`      // metadata_section only; unused for every other table
}

// SectionSyntax represents a PSI section syntax
type SectionSyntax struct {
	Data   SectionSyntaxData   `json:"_data"`
	Header SectionSyntaxHeader `json:"_header"`
}

// SectionSyntaxHeader represents a PSI section syntax header
type SectionSyntaxHeader struct {
	CurrentNextIndicator bool   `json:"current_next_indicator"` // Indicates if data is current in effect or is for future use. If the bit is flagged on, then the data is to be used at the present moment.
	LastSectionNumber    uint8  `json:"last_section_number"`    // This indicates which table is the last table in the sequence of tables.
	SectionNumber        uint8  `json:"section_number"`         // This is an index indicating which table this is in a related sequence of tables. The first table starts from 0.
	VersionNumber        uint8  `json:"version_number"`         // Syntax version number. Incremented when data is changed and wrapped around on overflow for values greater than 32.
	TableIDExtension     uint16 `json:"table_id_extension"`     // Informational only identifier. The PAT uses this for the transport stream identifier and the PMT uses this for the Program number.
}

// SectionSyntaxData represents a PSI section syntax data
type SectionSyntaxData any

// Parse parses a PSI data
func Parse(bs []byte) (d *Data, err error) {
	i := bytesiter.New(bs)

	d = &Data{}

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	d.PointerField = int(b)

	i.Skip(d.PointerField)

	var s Section
	var stop bool
	for i.HasBytesLeft() {
		if s, stop, err = parsePSISection(i); err != nil {
			err = fmt.Errorf("astits: parsing PSI table failed: %w", err)
			return
		}
		if stop {
			break
		}
		d.Sections = append(d.Sections, s)
	}
	return
}

// parsePSISection parses a PSI section
func parsePSISection(i *bytesiter.Iterator) (s Section, stop bool, err error) {
	var offsets psiOffsets
	if offsets, stop, err = s.Header.parsePSISectionHeader(i); err != nil {
		err = fmt.Errorf("astits: parsing PSI section header failed: %w", err)
		return
	}

	if stop {
		return
	}

	if s.Header.SectionLength > 0 {
		if s.Syntax, err = parsePSISectionSyntax(i, &s.Header, offsets.sectionsEnd); err != nil {
			err = fmt.Errorf("astits: parsing PSI section syntax failed: %w", err)
			return
		}

		if s.Header.TableID.hasCRC32() {
			i.Seek(offsets.sectionsEnd)

			if s.CRC32, err = parseCRC32(i); err != nil {
				err = fmt.Errorf("astits: parsing CRC32 failed: %w", err)
				return
			}

			i.Seek(offsets.start)
			var crc32Data []byte
			if crc32Data, err = i.NextBytesNoCopy(offsets.sectionsEnd - offsets.start); err != nil {
				err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
				return
			}

			crc32 := ts.ComputeCRC32(crc32Data)

			if crc32 != s.CRC32 {
				err = fmt.Errorf("astits: table CRC32 %x != computed CRC32 %x: %w", s.CRC32, crc32, ErrCRC32Mismatch)
				return
			}
		}
	}

	i.Seek(offsets.end)
	return
}

// parseCRC32 parses a CRC32
func parseCRC32(i *bytesiter.Iterator) (c uint32, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	c = binary.BigEndian.Uint32(bs)
	return
}

// StopsParsing reports whether sections from this table id on are stuffing:
// parsing must stop there. Besides 0xFF stuffing, an unrecognized table_id is
// treated as end-of-known-data: this demuxer only surfaces known tables, and
// stopping conservatively avoids mis-reading padding/torn tails as a section.
func (t TableID) StopsParsing() bool {
	return t == TableIDNull || t.IsUnknown()
}

type psiOffsets struct {
	start, end                 int
	sectionsStart, sectionsEnd int
}

// parsePSISectionHeader parses a PSI section header
func (h *SectionHeader) parsePSISectionHeader(i *bytesiter.Iterator) (offsets psiOffsets, stop bool, err error) {
	offsets.start = i.Offset()

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	h.TableID = TableID(b)

	if stop = h.TableID.StopsParsing(); stop {
		return
	}

	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	val := binary.BigEndian.Uint16(bs)
	h.SectionSyntaxIndicator = val&0x8000 > 0

	h.PrivateBit = val&0x4000 > 0

	if h.TableID == TableIDMetadata {
		h.RandomAccessIndicator = val&0x2000 > 0
		h.DecoderConfigFlag = val&0x1000 > 0
	}

	h.SectionLength = val & 0xfff

	offsets.sectionsStart = i.Offset()
	offsets.end = offsets.sectionsStart + int(h.SectionLength)
	offsets.sectionsEnd = offsets.end
	if h.TableID.hasCRC32() {
		offsets.sectionsEnd -= 4
	}
	if offsets.sectionsEnd < offsets.sectionsStart {
		err = fmt.Errorf("astits: section length %d is too short: %w", h.SectionLength, ts.ErrInvalidData)
	}
	return
}

// TableID.Type() returns the psi table type based on the table id
// Page: 28 | https://www.dvb.org/resources/public/standards/a38_dvb-si_specification.pdf
// (barbashov) the link above can be broken, alternative: https://dvb.org/wp-content/uploads/2019/12/a038_tm1217r37_en300468v1_17_1_-_rev-134_-_si_specification.pdf
func (t TableID) Type() string {
	switch {
	case t == TableIDBAT:
		return TableTypeBAT
	case t == TableIDCAT:
		return TableTypeCAT
	case t >= TableIDEITStart && t <= TableIDEITEnd:
		return TableTypeEIT
	case t == TableIDDIT:
		return TableTypeDIT
	case t == TableIDNITVariant1, t == TableIDNITVariant2:
		return TableTypeNIT
	case t == TableIDNull:
		return TableTypeNull
	case t == TableIDPAT:
		return TableTypePAT
	case t == TableIDPMT:
		return TableTypePMT
	case t == TableIDTSDT:
		return TableTypeTSDT
	case t == TableIDISO14496Scene, t == TableIDISO14496Object, t == TableIDISO14496:
		return TableTypeISO14496
	case t == TableIDMetadata:
		return TableTypeMetadata
	case t == TableIDRST:
		return TableTypeRST
	case t == TableIDSDTVariant1, t == TableIDSDTVariant2:
		return TableTypeSDT
	case t == TableIDSIT:
		return TableTypeSIT
	case t == TableIDST:
		return TableTypeST
	case t == TableIDTDT:
		return TableTypeTDT
	case t == TableIDTOT:
		return TableTypeTOT
	default:
		return TableTypeUnknown
	}
}

// hasPSISyntaxHeader checks whether the section has a syntax header
func (t TableID) hasPSISyntaxHeader() bool {
	return t == TableIDPAT ||
		t == TableIDCAT ||
		t == TableIDPMT ||
		t == TableIDTSDT ||
		t == TableIDBAT ||
		t == TableIDNITVariant1 || t == TableIDNITVariant2 ||
		t == TableIDSDTVariant1 || t == TableIDSDTVariant2 ||
		t == TableIDSIT ||
		t == TableIDISO14496Scene || t == TableIDISO14496Object || t == TableIDISO14496 ||
		(t >= TableIDEITStart && t <= TableIDEITEnd)
}

// hasCRC32 checks whether the table has a CRC32
func (t TableID) hasCRC32() bool {
	return t.hasPSISyntaxHeader() || t == TableIDTOT || t == TableIDMetadata
}

func (t TableID) IsUnknown() bool {
	switch t {
	case TableIDBAT,
		TableIDCAT,
		TableIDDIT,
		TableIDNITVariant1, TableIDNITVariant2,
		TableIDNull,
		TableIDPAT,
		TableIDPMT,
		TableIDTSDT,
		TableIDISO14496Scene, TableIDISO14496Object, TableIDISO14496,
		TableIDMetadata,
		TableIDRST,
		TableIDSDTVariant1, TableIDSDTVariant2,
		TableIDSIT,
		TableIDST,
		TableIDTDT,
		TableIDTOT:
		return false
	}
	if t >= TableIDEITStart && t <= TableIDEITEnd {
		return false
	}
	return true
}

// parsePSISectionSyntax parses a PSI section syntax
func parsePSISectionSyntax(i *bytesiter.Iterator, h *SectionHeader, offsetSectionsEnd int) (s *SectionSyntax, err error) {
	s = &SectionSyntax{}

	if h.TableID.hasPSISyntaxHeader() {
		if err = s.Header.parsePSISectionSyntaxHeader(i); err != nil {
			err = fmt.Errorf("astits: parsing PSI section syntax header failed: %w", err)
			return
		}
	}

	if s.Data, err = parsePSISectionSyntaxData(i, h, &s.Header, offsetSectionsEnd); err != nil {
		err = fmt.Errorf("astits: parsing PSI section syntax data failed: %w", err)
		return
	}
	return
}

// parsePSISectionSyntaxHeader parses a PSI section syntax header
func (h *SectionSyntaxHeader) parsePSISectionSyntaxHeader(i *bytesiter.Iterator) (err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	h.TableIDExtension = binary.BigEndian.Uint16(bs)

	// Get next byte
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	h.VersionNumber = b & 0x3f >> 1

	h.CurrentNextIndicator = b&0x1 > 0

	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	h.SectionNumber = b

	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	h.LastSectionNumber = b
	return
}

// parsePSISectionSyntaxData parses a PSI section data
func parsePSISectionSyntaxData(i *bytesiter.Iterator, h *SectionHeader, sh *SectionSyntaxHeader, offsetSectionsEnd int) (d SectionSyntaxData, err error) {
	switch h.TableID {
	case TableIDBAT:
		if d, err = parseBATSection(i, sh.TableIDExtension); err != nil {
			err = fmt.Errorf("astits: parsing BAT section failed: %w", err)
			return
		}
	case TableIDDIT:
		if d, err = parseDITSection(i); err != nil {
			err = fmt.Errorf("astits: parsing DIT section failed: %w", err)
			return
		}
	case TableIDNITVariant1, TableIDNITVariant2:
		if d, err = parseNITSection(i, sh.TableIDExtension); err != nil {
			err = fmt.Errorf("astits: parsing NIT section failed: %w", err)
			return
		}
	case TableIDPAT:
		if d, err = parsePATSection(i, offsetSectionsEnd, sh.TableIDExtension); err != nil {
			err = fmt.Errorf("astits: parsing PAT section failed: %w", err)
			return
		}
	case TableIDCAT:
		if d, err = parseCATSection(i, offsetSectionsEnd); err != nil {
			err = fmt.Errorf("astits: parsing CAT section failed: %w", err)
			return
		}
	case TableIDPMT:
		if d, err = parsePMTSection(i, offsetSectionsEnd, sh.TableIDExtension); err != nil {
			err = fmt.Errorf("astits: parsing PMT section failed: %w", err)
			return
		}
	case TableIDTSDT:
		if d, err = parseTSDTSection(i, offsetSectionsEnd); err != nil {
			err = fmt.Errorf("astits: parsing TSDT section failed: %w", err)
			return
		}
	case TableIDISO14496Scene, TableIDISO14496Object, TableIDISO14496:
		if d, err = parseISO14496Section(i, offsetSectionsEnd); err != nil {
			err = fmt.Errorf("astits: parsing ISO_IEC_14496 section failed: %w", err)
			return
		}
	case TableIDMetadata:
		if d, err = parseMetadataSection(i, offsetSectionsEnd); err != nil {
			err = fmt.Errorf("astits: parsing metadata section failed: %w", err)
			return
		}
	case TableIDRST:
		if d, err = parseRSTSection(i, offsetSectionsEnd); err != nil {
			err = fmt.Errorf("astits: parsing RST section failed: %w", err)
			return
		}
	case TableIDSDTVariant1, TableIDSDTVariant2:
		if d, err = parseSDTSection(i, offsetSectionsEnd, sh.TableIDExtension); err != nil {
			err = fmt.Errorf("astits: parsing PMT section failed: %w", err)
			return
		}
	case TableIDSIT:
		if d, err = parseSITSection(i, offsetSectionsEnd); err != nil {
			err = fmt.Errorf("astits: parsing SIT section failed: %w", err)
			return
		}
	case TableIDST:
		d = parseSTSection()
	case TableIDTOT:
		if d, err = parseTOTSection(i); err != nil {
			err = fmt.Errorf("astits: parsing TOT section failed: %w", err)
			return
		}
	case TableIDTDT:
		if d, err = parseTDTSection(i); err != nil {
			err = fmt.Errorf("astits: parsing TDT section failed: %w", err)
			return
		}
	}

	if h.TableID >= TableIDEITStart && h.TableID <= TableIDEITEnd {
		if d, err = parseEITSection(i, offsetSectionsEnd, sh.TableIDExtension); err != nil {
			err = fmt.Errorf("astits: parsing EIT section failed: %w", err)
			return
		}
	}

	return
}

// Append appends the serialized PSI data (pointer field, sections) to dst.
func (d *Data) Append(dst []byte) ([]byte, error) {
	dst = append(dst, uint8(d.PointerField))
	for i := 0; i < d.PointerField; i++ {
		dst = append(dst, 0x00)
	}

	var err error
	for i := range d.Sections {
		if dst, err = d.Sections[i].appendSection(dst); err != nil {
			return dst, err
		}
	}

	return dst, nil
}

// sectionBody is a PSI table that serializes its section body — the bytes between
// the syntax header and the CRC. Every serializable table implements it; a table
// whose Data does not is reported as ErrTableNotImplemented.
type sectionBody interface {
	CalcSectionLength() int
	appendSection(dst []byte) []byte
}

func (s *Section) calcPSISectionLength(body sectionBody) (ret uint16) {
	if s.Header.TableID.hasPSISyntaxHeader() {
		ret += 5 // PSI syntax header length
	}
	ret += uint16(body.CalcSectionLength())
	if s.Header.TableID.hasCRC32() {
		ret += 4
	}
	return ret
}

func (s *Section) appendSection(dst []byte) ([]byte, error) {
	// A zero-length section (a parsed stuffing table) carries no syntax; anything
	// with a body, syntax header or CRC has a non-nil Syntax.
	var body sectionBody
	if s.Syntax != nil {
		var ok bool
		if body, ok = s.Syntax.Data.(sectionBody); !ok {
			return dst, fmt.Errorf("astits: appending table %s: %w", s.Header.TableID.Type(), ErrTableNotImplemented)
		}
	}

	var sectionLength uint16
	if body != nil {
		sectionLength = s.calcPSISectionLength(body)
	}
	if sectionLength > maxSectionLength {
		return dst, fmt.Errorf("astits: section length %d exceeds %d: %w", sectionLength, maxSectionLength, ErrSectionOverflow)
	}
	crcStart := len(dst)

	reserved := byte(0x30) // both reserved bits set, except metadata_section carries flags here
	if s.Header.TableID == TableIDMetadata {
		reserved = util.B2U(s.Header.RandomAccessIndicator)<<5 | util.B2U(s.Header.DecoderConfigFlag)<<4
	}
	dst = append(dst, uint8(s.Header.TableID))
	dst = append(dst,
		util.B2U(s.Header.SectionSyntaxIndicator)<<7|util.B2U(s.Header.PrivateBit)<<6|reserved|byte(sectionLength>>8)&0xf,
		byte(sectionLength))

	// A zero-length section has no syntax header, body or CRC to follow; anything
	// with a syntax header or CRC (long form) is non-zero even with an empty body.
	if sectionLength > 0 {
		if s.Header.TableID.hasPSISyntaxHeader() {
			dst = s.Syntax.Header.appendSectionSyntaxHeader(dst)
		}
		dst = body.appendSection(dst)

		if s.Header.TableID.hasCRC32() {
			crc := ts.UpdateCRC32(ts.CRC32Seed, dst[crcStart:])
			dst = append(dst, byte(crc>>24), byte(crc>>16), byte(crc>>8), byte(crc))
		}
	}

	return dst, nil
}

func (h *SectionSyntaxHeader) appendSectionSyntaxHeader(dst []byte) []byte {
	return append(dst,
		byte(h.TableIDExtension>>8), byte(h.TableIDExtension),
		0xc0|h.VersionNumber&0x1f<<1|util.B2U(h.CurrentNextIndicator),
		h.SectionNumber,
		h.LastSectionNumber)
}
