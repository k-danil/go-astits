package psi

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/k-danil/go-astits/v2/internal/bitstest"
	"github.com/k-danil/go-astits/v2/internal/bytesiter"
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
		//{Header: SectionHeader{
		//	TableID: 254,
		//}},
	},
}

func psiBytes() []byte {
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
	_ = w.Write(uint8(254))                    // Unknown table ID
	_ = w.Write(uint8(0))                      // PAT table ID
	return buf.Bytes()
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
	_, err := Parse(buf.Bytes())
	assert.EqualError(t, err, "astits: parsing PSI table failed: astits: Table CRC32 20 != computed CRC32 6969b13")

	// Valid
	d, err := Parse(psiBytes())
	assert.NoError(t, err)
	assert.Equal(t, d, psi)
}

var psiSectionHeader = SectionHeader{
	PrivateBit:             true,
	SectionLength:          2730,
	SectionSyntaxIndicator: true,
	TableID:                0,
}

func psiSectionHeaderBytes() []byte {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write(uint8(0))       // Table ID
	_ = w.Write("1")            // Syntax section indicator
	_ = w.Write("1")            // Private bit
	_ = w.Write("11")           // Reserved
	_ = w.Write("101010101010") // Section length
	return buf.Bytes()
}

func TestParsePSISectionHeader(t *testing.T) {
	// Unknown table type
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write(uint8(254)) // Table ID
	_ = w.Write("1")        // Syntax section indicator
	_ = w.Write("0000000")  // Finish the byte
	var d SectionHeader
	_, _, err := d.parsePSISectionHeader(bytesiter.New(buf.Bytes()))
	assert.Equal(t, d, SectionHeader{
		TableID: 254,
	})
	assert.NoError(t, err)

	d = SectionHeader{}
	// Valid table type
	offsets, _, err := d.parsePSISectionHeader(bytesiter.New(psiSectionHeaderBytes()))
	assert.Equal(t, d, psiSectionHeader)
	assert.Equal(t, 0, offsets.start)
	assert.Equal(t, 3, offsets.sectionsStart)
	assert.Equal(t, 2729, offsets.sectionsEnd)
	assert.Equal(t, 2733, offsets.end)
	assert.NoError(t, err)
}

func TestPSITableType(t *testing.T) {
	for i := TableIDEITStart; i <= TableIDEITEnd; i++ {
		assert.Equal(t, TableTypeEIT, i.Type())
	}
	assert.Equal(t, TableTypeDIT, TableIDDIT.Type())
	assert.Equal(t, TableTypeNIT, TableIDNITVariant1.Type())
	assert.Equal(t, TableTypeNIT, TableIDNITVariant2.Type())
	assert.Equal(t, TableTypeSDT, TableIDSDTVariant1.Type())
	assert.Equal(t, TableTypeSDT, TableIDSDTVariant2.Type())

	assert.Equal(t, TableTypeBAT, TableIDBAT.Type())
	assert.Equal(t, TableTypeNull, TableIDNull.Type())
	assert.Equal(t, TableTypePAT, TableIDPAT.Type())
	assert.Equal(t, TableTypePMT, TableIDPMT.Type())
	assert.Equal(t, TableTypeRST, TableIDRST.Type())
	assert.Equal(t, TableTypeSIT, TableIDSIT.Type())
	assert.Equal(t, TableTypeST, TableIDST.Type())
	assert.Equal(t, TableTypeTDT, TableIDTDT.Type())
	assert.Equal(t, TableTypeTOT, TableIDTOT.Type())
	assert.Equal(t, TableTypeUnknown, TableID(1).Type())
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

func TestParsePSISectionSyntaxHeader(t *testing.T) {
	var h SectionSyntaxHeader
	err := h.parsePSISectionSyntaxHeader(bytesiter.New(psiSectionSyntaxHeaderBytes()))
	assert.Equal(t, psiSectionSyntaxHeader, h)
	assert.NoError(t, err)
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
			assert.NoError(t, err)
			assert.Equal(t, bufExpected.Len(), len(actual))
			assert.Equal(t, bufExpected.Bytes(), actual)
		})
	}
}

func BenchmarkParsePSIData(b *testing.B) {
	pb := psiBytes()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Parse(pb)
	}
}
