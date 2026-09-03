package psi

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v3/internal/bitstest"
	"github.com/k-danil/go-astits/v3/ts"
)

var psi = &Data{
	PointerField: 4,
	Sections: []Section{
		{
			CRC32: uint32(0x7ffc6102),
			Header: SectionHeader{
				PrivateBit:             true,
				SectionLength:          30,
				SectionSyntaxIndicator: true,
				TableID:                78,
			},
			Syntax: &SectionSyntax{
				Data:   eit,
				Header: psiSectionSyntaxHeader,
			},
		},
		{
			CRC32: uint32(0xfebaa941),
			Header: SectionHeader{
				PrivateBit:             true,
				SectionLength:          25,
				SectionSyntaxIndicator: true,
				TableID:                64,
			},
			Syntax: &SectionSyntax{
				Data:   nit,
				Header: psiSectionSyntaxHeader,
			},
		},
		{
			CRC32: uint32(0x60739f61),
			Header: SectionHeader{
				PrivateBit:             true,
				SectionLength:          17,
				SectionSyntaxIndicator: true,
				TableID:                0,
			},
			Syntax: &SectionSyntax{
				Data:   pat,
				Header: psiSectionSyntaxHeader,
			},
		},
		{
			CRC32: uint32(0xc68442e8),
			Header: SectionHeader{
				PrivateBit:             true,
				SectionLength:          24,
				SectionSyntaxIndicator: true,
				TableID:                2,
			},
			Syntax: &SectionSyntax{
				Data:   pmt,
				Header: psiSectionSyntaxHeader,
			},
		},
		{
			CRC32: uint32(0xef3751d6),
			Header: SectionHeader{
				PrivateBit:             true,
				SectionLength:          20,
				SectionSyntaxIndicator: true,
				TableID:                66,
			},
			Syntax: &SectionSyntax{
				Data:   sdt,
				Header: psiSectionSyntaxHeader,
			},
		},
		{
			CRC32: uint32(0x6969b13),
			Header: SectionHeader{
				PrivateBit:             true,
				SectionLength:          14,
				SectionSyntaxIndicator: true,
				TableID:                115,
			},
			Syntax: &SectionSyntax{
				Data: tot,
			},
		},
	},
}

func psiSectionsBytes() []byte {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write(uint8(4))                      // Pointer field
	_ = w.Write([]byte("test"))                // Pointer field bytes
	_ = w.Write(uint8(78))                     // EIT table ID
	_ = w.Write("1")                           // EIT syntax section indicator
	_ = w.Write("1")                           // EIT private bit
	_ = w.Write("11")                          // EIT reserved
	_ = w.Write("000000011110")                // EIT section length
	_ = w.Write(psiSectionSyntaxHeaderBytes()) // EIT syntax section header
	_ = w.Write(eitBytes())                    // EIT data
	_ = w.Write(uint32(0x7ffc6102))            // EIT CRC32
	_ = w.Write(uint8(64))                     // NIT table ID
	_ = w.Write("1")                           // NIT syntax section indicator
	_ = w.Write("1")                           // NIT private bit
	_ = w.Write("11")                          // NIT reserved
	_ = w.Write("000000011001")                // NIT section length
	_ = w.Write(psiSectionSyntaxHeaderBytes()) // NIT syntax section header
	_ = w.Write(nitBytes())                    // NIT data
	_ = w.Write(uint32(0xfebaa941))            // NIT CRC32
	_ = w.Write(uint8(0))                      // PAT table ID
	_ = w.Write("1")                           // PAT syntax section indicator
	_ = w.Write("1")                           // PAT private bit
	_ = w.Write("11")                          // PAT reserved
	_ = w.Write("000000010001")                // PAT section length
	_ = w.Write(psiSectionSyntaxHeaderBytes()) // PAT syntax section header
	_ = w.Write(patBytes())                    // PAT data
	_ = w.Write(uint32(0x60739f61))            // PAT CRC32
	_ = w.Write(uint8(2))                      // PMT table ID
	_ = w.Write("1")                           // PMT syntax section indicator
	_ = w.Write("1")                           // PMT private bit
	_ = w.Write("11")                          // PMT reserved
	_ = w.Write("000000011000")                // PMT section length
	_ = w.Write(psiSectionSyntaxHeaderBytes()) // PMT syntax section header
	_ = w.Write(pmtBytes())                    // PMT data
	_ = w.Write(uint32(0xc68442e8))            // PMT CRC32
	_ = w.Write(uint8(66))                     // SDT table ID
	_ = w.Write("1")                           // SDT syntax section indicator
	_ = w.Write("1")                           // SDT private bit
	_ = w.Write("11")                          // SDT reserved
	_ = w.Write("000000010100")                // SDT section length
	_ = w.Write(psiSectionSyntaxHeaderBytes()) // SDT syntax section header
	_ = w.Write(sdtBytes())                    // SDT data
	_ = w.Write(uint32(0xef3751d6))            // SDT CRC32
	_ = w.Write(uint8(115))                    // TOT table ID
	_ = w.Write("1")                           // TOT syntax section indicator
	_ = w.Write("1")                           // TOT private bit
	_ = w.Write("11")                          // TOT reserved
	_ = w.Write("000000001110")                // TOT section length
	_ = w.Write(totBytes())                    // TOT data
	_ = w.Write(uint32(0x6969b13))             // TOT CRC32
	return buf.Bytes()
}

// psiBytes ends the sections with an unknown table_id: the stop marker Parse
// reports as a section error.
func psiBytes() []byte {
	return append(psiSectionsBytes(), 254, 0)
}

func TestParsePSIData(t *testing.T) {
	// Invalid CRC32
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write(uint8(0))       // Pointer field
	_ = w.Write(uint8(115))     // TOT table ID
	_ = w.Write("1")            // TOT syntax section indicator
	_ = w.Write("1")            // TOT private bit
	_ = w.Write("11")           // TOT reserved
	_ = w.Write("000000001110") // TOT section length
	_ = w.Write(totBytes())     // TOT data
	_ = w.Write(uint32(32))     // TOT CRC32
	d, err := Parse(buf.Bytes())
	require.NoError(t, err)
	assert.Empty(t, d.Sections)
	require.Len(t, d.Errors, 1)
	require.ErrorIs(t, d.Errors[0], ErrCRC32Mismatch)
	require.ErrorIs(t, d.Errors[0], ts.ErrInvalidData)

	// Valid, ending in an unknown table_id that is reported rather than swallowed
	d, err = Parse(psiBytes())
	require.NoError(t, err)
	assert.Equal(t, psi.PointerField, d.PointerField)
	assert.Equal(t, psi.Sections, d.Sections)
	require.Len(t, d.Errors, 1)
	assert.Equal(t, &SectionError{TableID: 254, Offset: 153, Len: 2, Err: ErrUnknownTable}, d.Errors[0])
}

func TestPSITableType(t *testing.T) {
	for i := range 256 {
		id := TableID(uint8(i))
		assert.Equal(t, id.IsUnknown(), id.Type() == TableTypeUnknown, "0x%02x", i)
	}
}

var psiSectionSyntaxHeader = SectionSyntaxHeader{
	CurrentNextIndicator: true,
	LastSectionNumber:    3,
	SectionNumber:        2,
	TableIDExtension:     1,
	VersionNumber:        21,
}

func psiSectionSyntaxHeaderBytes() []byte {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write(uint16(1)) // Table ID extension
	_ = w.Write("11")      // Reserved bits
	_ = w.Write("10101")   // Version number
	_ = w.Write("1")       // Current/next indicator
	_ = w.Write(uint8(2))  // Section number
	_ = w.Write(uint8(3))  // Last section number
	return buf.Bytes()
}

type psiDataTestCase struct {
	name      string
	bytesFunc func(*bitstest.Writer)
	data      *Data
}

var psiDataTestCases = []psiDataTestCase{
	{
		"PAT",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(4))                      // Pointer field
			_ = w.Write([]byte{0, 0, 0, 0})            // Pointer field bytes
			_ = w.Write(uint8(0))                      // PAT table ID
			_ = w.Write("1")                           // PAT syntax section indicator
			_ = w.Write("1")                           // PAT private bit
			_ = w.Write("11")                          // PAT reserved
			_ = w.Write("000000010001")                // PAT section length
			_ = w.Write(psiSectionSyntaxHeaderBytes()) // PAT syntax section header
			_ = w.Write(patBytes())                    // PAT data
			_ = w.Write(uint32(0x60739f61))            // PAT CRC32
		},
		&Data{
			PointerField: 4,
			Sections: []Section{
				{
					CRC32: uint32(0x60739f61),
					Header: SectionHeader{
						PrivateBit:             true,
						SectionLength:          17,
						SectionSyntaxIndicator: true,
						TableID:                0,
					},
					Syntax: &SectionSyntax{
						Data:   pat,
						Header: psiSectionSyntaxHeader,
					},
				},
			},
		},
	},
	{
		"PMT",
		func(w *bitstest.Writer) {
			_ = w.Write(uint8(4))                      // Pointer field
			_ = w.Write([]byte{0, 0, 0, 0})            // Pointer field bytes
			_ = w.Write(uint8(2))                      // PMT table ID
			_ = w.Write("1")                           // PMT syntax section indicator
			_ = w.Write("1")                           // PMT private bit
			_ = w.Write("11")                          // PMT reserved
			_ = w.Write("000000011000")                // PMT section length
			_ = w.Write(psiSectionSyntaxHeaderBytes()) // PMT syntax section header
			_ = w.Write(pmtBytes())                    // PMT data
			_ = w.Write(uint32(0xc68442e8))            // PMT CRC32
		},
		&Data{
			PointerField: 4,
			Sections: []Section{
				{
					CRC32: uint32(0xc68442e8),
					Header: SectionHeader{
						PrivateBit:             true,
						SectionLength:          24,
						SectionSyntaxIndicator: true,
						TableID:                2,
					},
					Syntax: &SectionSyntax{
						Data:   pmt,
						Header: psiSectionSyntaxHeader,
					},
				},
			},
		},
	},
}

func TestWritePSIData(t *testing.T) {
	for _, tc := range psiDataTestCases {
		t.Run(tc.name, func(t *testing.T) {
			bufExpected := bytes.Buffer{}
			wExpected := bitstest.NewWriter(&bufExpected)

			tc.bytesFunc(wExpected)

			actual, err := tc.data.Append(nil)
			require.NoError(t, err)
			assert.Len(t, actual, bufExpected.Len())
			assert.Equal(t, bufExpected.Bytes(), actual)
		})
	}
}

func BenchmarkParsePSIData(b *testing.B) {
	// The successful path over the same sections as the reference number; the
	// fixture's unknown-table trailer is a reported error, benchmarked nowhere
	pb := psiSectionsBytes()
	b.ReportAllocs()
	for b.Loop() {
		_, _ = Parse(pb)
	}
}
