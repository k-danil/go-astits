package psi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Reserved bits are written as ones (H.222.0 §2.4.4, EN 300 468 §5.2): a
// round trip cannot see them, and a receiver may reject a section that has
// them cleared.
func TestReservedBitsOnWrite(t *testing.T) {
	tests := []struct {
		name   string
		unit   Data
		offset int  // byte within the appended unit (pointer field included)
		want   byte // value with every other field zero
	}{
		{"SDT service: 6 reserved bits ahead of the EIT flags", Data{Sections: []Section{{
			Header: SectionHeader{TableID: TableIDSDTVariant1, SectionSyntaxIndicator: true},
			Syntax: &SectionSyntax{Data: &SDT{Services: []SDTService{{}}}},
		}}}, 14, 0xfc},
		{"RST event: 5 reserved bits ahead of running_status", Data{Sections: []Section{{
			Header: SectionHeader{TableID: TableIDRST},
			Syntax: &SectionSyntax{Data: &RST{Events: []RSTEvent{{}}}},
		}}}, 12, 0xf8},
		{"DIT: 7 reserved bits after transition_flag", Data{Sections: []Section{{
			Header: SectionHeader{TableID: TableIDDIT},
			Syntax: &SectionSyntax{Data: &DIT{}},
		}}}, 4, 0x7f},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bs, err := tt.unit.Append(nil)
			require.NoError(t, err)
			require.Greater(t, len(bs), tt.offset)
			assert.Equal(t, tt.want, bs[tt.offset], "%x", bs)
		})
	}
}
