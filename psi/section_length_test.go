package psi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The cap follows the table: private_section tables (EIT included) run past 1021.
func TestAppendSectionLengthLimit(t *testing.T) {
	for _, tc := range []struct {
		name     string
		tableID  TableID
		data     SectionSyntaxData
		wantLen  int
		overflow bool
	}{
		{name: "PAT past 1021", tableID: TableIDPAT, data: &PAT{Programs: make([]PATProgram, 275)}, overflow: true},
		{name: "EIT past 1021", tableID: TableIDEITStart, data: &EIT{Events: make([]EITEvent, 320)}, wantLen: 3858},
		{name: "EIT past 4093", tableID: TableIDEITStart, data: &EIT{Events: make([]EITEvent, 340)}, overflow: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := Section{
				Header: SectionHeader{TableID: tc.tableID, SectionSyntaxIndicator: true},
				Syntax: &SectionSyntax{Data: tc.data},
			}
			bs, err := s.appendSection(nil)
			if tc.overflow {
				require.ErrorIs(t, err, ErrSectionOverflow)
				return
			}
			require.NoError(t, err)
			assert.Len(t, bs, tc.wantLen)
		})
	}
}
