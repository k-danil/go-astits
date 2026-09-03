package psi

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
	"github.com/k-danil/go-astits/v3/internal/errclass"
	"github.com/k-danil/go-astits/v3/internal/util"
	"github.com/k-danil/go-astits/v3/ts"
)

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

var ErrCRC32Mismatch = errclass.New("astits: CRC32 mismatch", ts.ErrInvalidData)

var (
	ErrPointerField = errclass.New("astits: pointer_field beyond the unit", ts.ErrInvalidData)
	ErrNoSections   = errclass.New("astits: unit carries no section", ts.ErrInvalidData)
	ErrUnknownTable = errclass.New("astits: unknown table_id", ts.ErrInvalidData)
)

var ErrTableNotImplemented = errors.New("astits: table serialization is not implemented")

var ErrSectionOverflow = errors.New("astits: section data does not fit a single section")

// ISO/IEC 13818-1 §2.4.4.1: 12-bit field, capped at 1021 for PSI sections.
const (
	maxSectionLength    = 1021
	psiSyntaxHeaderLen  = 5
	crc32Len            = 4
	sectionReservedBits = 0x30
)

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

type Data struct {
	PointerField int       `json:"pointer_field"`
	Sections     []Section `json:"_sections"`
	// Errors lists the sections that could not be used; Sections holds the rest. Parse itself fails only when the unit yielded neither.
	Errors []*SectionError `json:"-"`
}

type SectionError struct {
	Err     error
	Offset  int
	Len     int // bytes lost with it, up to the end of the unit when the length itself is unusable
	TableID TableID
}

func (e *SectionError) Error() string {
	return fmt.Sprintf("astits: section %s at %d (%d bytes): %v", e.TableID, e.Offset, e.Len, e.Err)
}

func (e *SectionError) Unwrap() error { return e.Err }

type Section struct {
	Syntax *SectionSyntax `json:"_syntax"`
	CRC32  uint32         `json:"_crc32"`
	Header SectionHeader  `json:"_header"`
}

type SectionHeader struct {
	SectionLength          uint16  `json:"section_length"`
	TableID                TableID `json:"table_id"`
	SectionSyntaxIndicator bool    `json:"section_syntax_indicator"`
	PrivateBit             bool    `json:"private_indicator"`
	// Set only for metadata_section; reserved bits in every other table.
	RandomAccessIndicator bool `json:"random_access_indicator"`
	DecoderConfigFlag     bool `json:"decoder_config_flag"`
}

type SectionSyntax struct {
	Data   SectionSyntaxData   `json:"_data"`
	Header SectionSyntaxHeader `json:"_header"`
}

type SectionSyntaxHeader struct {
	CurrentNextIndicator bool   `json:"current_next_indicator"`
	LastSectionNumber    uint8  `json:"last_section_number"`
	SectionNumber        uint8  `json:"section_number"`
	VersionNumber        uint8  `json:"version_number"`
	TableIDExtension     uint16 `json:"table_id_extension"`
}

type SectionSyntaxData any

func Parse(bs []byte) (d *Data, err error) {
	i := bytesiter.New(bs)

	d = &Data{}

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	d.PointerField = int(b)

	if d.PointerField > i.Len()-i.Offset() {
		err = fmt.Errorf("astits: pointer_field %d exceeds the %d bytes left: %w", d.PointerField, i.Len()-i.Offset(), ErrPointerField)
		return
	}
	i.Skip(d.PointerField)

	for i.HasBytesLeft() {
		s, serr, stop := parsePSISection(i)
		if serr != nil {
			d.Errors = append(d.Errors, serr)
		} else if !stop {
			d.Sections = append(d.Sections, s)
		}
		if stop {
			break
		}
	}
	if len(d.Sections) == 0 && len(d.Errors) == 0 {
		err = ErrNoSections
	}
	return
}

// stop: nothing more to parse — stuffing, an unknown table, or a length too damaged to skip over.
func parsePSISection(i *bytesiter.Iterator) (s Section, serr *SectionError, stop bool) {
	start := i.Offset()

	var offsets psiOffsets
	var err error
	if offsets, stop, err = s.Header.parsePSISectionHeader(i); err != nil {
		serr = &SectionError{TableID: s.Header.TableID, Offset: start, Len: i.Len() - start, Err: fmt.Errorf("astits: parsing PSI section header failed: %w", err)}
		stop = true
		return
	}
	if stop {
		if s.Header.TableID != TableIDNull {
			serr = &SectionError{TableID: s.Header.TableID, Offset: start, Len: i.Len() - start, Err: ErrUnknownTable}
		}
		return
	}
	if offsets.end > i.Len() {
		err = fmt.Errorf("astits: section length %d exceeds the %d bytes left: %w", s.Header.SectionLength, i.Len()-offsets.sectionsStart, bytesiter.ErrNoBytesLeft)
		serr = &SectionError{TableID: s.Header.TableID, Offset: start, Len: i.Len() - start, Err: err}
		stop = true
		return
	}

	if s.Header.SectionLength > 0 {
		if err = s.parseBody(i, offsets); err != nil {
			serr = &SectionError{TableID: s.Header.TableID, Offset: start, Len: offsets.end - start, Err: err}
		}
	}

	i.Seek(offsets.end)
	return
}

// parseBody checks the CRC32 before parsing: a damaged length or body must
// count as a CRC error, not as whatever the body parser trips over.
func (s *Section) parseBody(i *bytesiter.Iterator, offsets psiOffsets) (err error) {
	if s.Header.TableID.hasCRC32() {
		i.Seek(offsets.sectionsEnd)
		if s.CRC32, err = parseCRC32(i); err != nil {
			return fmt.Errorf("astits: parsing CRC32 failed: %w", err)
		}

		i.Seek(offsets.start)
		var covered []byte
		if covered, err = i.NextBytesNoCopy(offsets.sectionsEnd - offsets.start); err != nil {
			return fmt.Errorf("astits: fetching next bytes failed: %w", err)
		}
		if crc32 := ts.ComputeCRC32(covered); crc32 != s.CRC32 {
			return fmt.Errorf("astits: table CRC32 %x != computed CRC32 %x: %w", s.CRC32, crc32, ErrCRC32Mismatch)
		}
	}

	i.Seek(offsets.sectionsStart)
	prev := i.Limit(offsets.sectionsEnd)
	s.Syntax, err = parsePSISectionSyntax(i, &s.Header, offsets.sectionsEnd)
	i.Limit(prev)
	if err != nil {
		return fmt.Errorf("astits: parsing PSI section syntax failed: %w", err)
	}
	return
}

func parseCRC32(i *bytesiter.Iterator) (c uint32, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	c = binary.BigEndian.Uint32(bs)
	return
}

// An unknown table_id is treated as end-of-data: stopping avoids reading padding or a torn tail as a section.
func (t TableID) StopsParsing() bool {
	return t == TableIDNull || t.IsUnknown()
}

type psiOffsets struct {
	start, end                 int
	sectionsStart, sectionsEnd int
}

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
		offsets.sectionsEnd -= crc32Len
	}
	if offsets.sectionsEnd < offsets.sectionsStart {
		err = fmt.Errorf("astits: section length %d is too short: %w", h.SectionLength, ts.ErrInvalidData)
	}
	return
}

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

func (h *SectionSyntaxHeader) parsePSISectionSyntaxHeader(i *bytesiter.Iterator) (err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	h.TableIDExtension = binary.BigEndian.Uint16(bs)

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

type sectionBody interface {
	CalcSectionLength() int
	appendSection(dst []byte) []byte
}

func (s *Section) calcPSISectionLength(body sectionBody) (ret uint16) {
	if s.Header.TableID.hasPSISyntaxHeader() {
		ret += psiSyntaxHeaderLen
	}
	ret += uint16(body.CalcSectionLength())
	if s.Header.TableID.hasCRC32() {
		ret += crc32Len
	}
	return ret
}

func (s *Section) appendSection(dst []byte) ([]byte, error) {
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

	reserved := byte(sectionReservedBits)
	if s.Header.TableID == TableIDMetadata {
		reserved = util.B2U(s.Header.RandomAccessIndicator)<<5 | util.B2U(s.Header.DecoderConfigFlag)<<4
	}
	dst = append(dst, uint8(s.Header.TableID))
	dst = append(dst,
		util.B2U(s.Header.SectionSyntaxIndicator)<<7|util.B2U(s.Header.PrivateBit)<<6|reserved|byte(sectionLength>>8)&0xf,
		byte(sectionLength))

	// body is nil exactly when sectionLength is 0 (a stuffing table): dropping this guard dereferences it.
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
