package psi

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
	"github.com/k-danil/go-astits/v3/ts"
)

// sectionBytes frames body (syntax header included) as a section with an
// honest CRC32.
func sectionBytes(id TableID, body []byte) []byte {
	l := len(body) + 4
	s := append([]byte{byte(id), 0xb0 | byte(l>>8), byte(l)}, body...)
	return binary.BigEndian.AppendUint32(s, ts.ComputeCRC32(s))
}

func patSection() []byte {
	return sectionBytes(TableIDPAT, append(psiSectionSyntaxHeaderBytes(), patBytes()...))
}

// pmtOverrunSection is a PMT whose only ES claims more descriptor bytes than
// the section has; the honest CRC32 makes it the encoder's lie, not transport
// damage. The transport_stream_id is searched so that, read across the
// section end, the CRC32 bytes form a descriptor header whose length matches
// the claim: only the section bound then keeps the parser off the next
// section's bytes.
func pmtOverrunSection() []byte {
	const maxDescriptorLen = 255
	for tsID := range 1 << 16 {
		for claim := 3; claim <= 2+maxDescriptorLen; claim++ {
			body := []byte{byte(tsID >> 8), byte(tsID), 0xc1, 0x00, 0x00, // syntax header, current
				0xe1, 0x00, // PCR PID 0x100
				0xf0, 0x00, // program_info_length 0
				0x1b, 0xe1, 0x00, // AVC on PID 0x100
				0xf0 | byte(claim>>8), byte(claim), // ES_info_length
			}
			s := sectionBytes(TableIDPMT, body)
			if int(s[len(s)-3]) == claim-2 {
				return s
			}
		}
	}
	panic("no transport_stream_id makes the CRC32 fit the claim")
}

// stSection is a 300-byte stuffing section: room for any descriptor a parser
// let loose past the previous section would claim.
func stSection() []byte {
	bodyLen := 300
	return append([]byte{byte(TableIDST), 0x70 | byte(bodyLen>>8), byte(bodyLen)}, make([]byte, bodyLen)...)
}

func TestParsePartialUnits(t *testing.T) {
	brokenPAT := patSection()
	brokenPAT[len(brokenPAT)-1] ^= 0x01
	brokenSAT := sectionBytes(TableIDSAT, append(psiSectionSyntaxHeaderBytes(), 0, 0, 0, 0))
	brokenSAT[len(brokenSAT)-1] ^= 0x01
	const longST = 1500
	damagedPMT := pmtOverrunSection()
	damagedPMT[len(damagedPMT)-1] ^= 0x01

	type sectionErr struct {
		is  error
		id  TableID
		len int
	}
	tests := []struct {
		name       string
		unit       []byte
		wantErr    error
		wantTables []TableID
		wantErrs   []sectionErr
	}{
		{
			name:     "CRC32 is checked before the body",
			unit:     append([]byte{0}, damagedPMT...),
			wantErrs: []sectionErr{{is: ErrCRC32Mismatch, id: TableIDPMT, len: len(damagedPMT)}},
		},
		{
			name:    "pointer_field beyond the unit",
			unit:    []byte{0x10, 0xff, 0xff},
			wantErr: ErrPointerField,
		},
		{
			name:    "stuffing only",
			unit:    []byte{0x00, 0xff, 0xff},
			wantErr: ErrNoSections,
		},
		{
			name:       "a broken section does not cost the next one",
			unit:       append(append([]byte{0}, brokenPAT...), patSection()...),
			wantTables: []TableID{TableIDPAT},
			wantErrs:   []sectionErr{{is: ErrCRC32Mismatch, id: TableIDPAT, len: len(brokenPAT)}},
		},
		{
			name:       "an unknown table is skipped by its length and the next section still parses",
			unit:       append([]byte{0, 0x74, 0x00, 0x02, 0xaa, 0xbb}, patSection()...),
			wantTables: []TableID{TableIDPAT},
			wantErrs:   []sectionErr{{is: ErrUnknownTable, id: 0x74, len: 5}},
		},
		{
			name:     "a PMT section_length past 1021 is a length error even when the unit holds it",
			unit:     append([]byte{0, 0x02, 0xb4, 0x4c}, make([]byte, 1200)...),
			wantErrs: []sectionErr{{is: ErrSectionLength, id: TableIDPMT, len: 1203}},
		},
		{
			name:       "a stuffing table longer than 1024 bytes is legal",
			unit:       append([]byte{0, byte(TableIDST), 0x70 | byte(longST>>8), byte(longST & 0xff)}, make([]byte, longST)...),
			wantTables: []TableID{TableIDST},
		},
		{
			name:     "a general-syntax section shorter than 9 bytes is a length error",
			unit:     []byte{0, 0x00, 0xb0, 0x05, 0, 0, 0, 0, 0},
			wantErrs: []sectionErr{{is: ErrSectionLength, id: TableIDPAT, len: 8}},
		},
		{
			name:     "a TOT shorter than its CRC32 is a length error",
			unit:     []byte{0, 0x73, 0x70, 0x02, 0, 0},
			wantErrs: []sectionErr{{is: ErrSectionLength, id: TableIDTOT, len: 5}},
		},
		{
			name:     "a SAT carries a CRC32",
			unit:     append([]byte{0}, brokenSAT...),
			wantErrs: []sectionErr{{is: ErrCRC32Mismatch, id: TableIDSAT, len: len(brokenSAT)}},
		},
		{
			name:       "a SAT longer than 1024 bytes is legal",
			unit:       satUnit(63, 0, make([]byte, longST)),
			wantTables: []TableID{TableIDSAT},
		},
		{
			name:       "descriptors never come from the next section",
			unit:       append(append([]byte{0}, pmtOverrunSection()...), stSection()...),
			wantTables: []TableID{TableIDST},
			wantErrs:   []sectionErr{{is: bytesiter.ErrNoBytesLeft, id: TableIDPMT, len: len(pmtOverrunSection())}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := Parse(tt.unit)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)

			var tables []TableID
			for _, s := range d.Sections {
				tables = append(tables, s.Header.TableID)
			}
			assert.Equal(t, tt.wantTables, tables)

			require.Len(t, d.Errors, len(tt.wantErrs))
			for i, want := range tt.wantErrs {
				assert.Equal(t, want.id, d.Errors[i].TableID)
				assert.Equal(t, want.len, d.Errors[i].Len)
				assert.ErrorIs(t, d.Errors[i], want.is)
			}
		})
	}
}
