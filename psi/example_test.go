package psi_test

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/psi"
)

// A PSI unit (pointer field plus sections) round-trips through Append and
// Parse; the typed table sits behind Syntax.Data.
func ExampleParse() {
	unit := psi.Data{Sections: []psi.Section{{
		Header: psi.SectionHeader{TableID: psi.TableIDPAT, SectionSyntaxIndicator: true},
		Syntax: &psi.SectionSyntax{
			Header: psi.SectionSyntaxHeader{TableIDExtension: 1, CurrentNextIndicator: true},
			Data:   &psi.PAT{TransportStreamID: 1, Programs: []psi.PATProgram{{ProgramNumber: 1, ProgramMapID: 0x1000}}},
		},
	}}}
	bs, err := unit.Append(nil)
	if err != nil {
		panic(err)
	}

	parsed, err := psi.Parse(bs)
	if err != nil {
		panic(err)
	}
	if pat, ok := parsed.Sections[0].Syntax.Data.(*psi.PAT); ok {
		fmt.Printf("%s: program %d at PMT PID 0x%x, CRC ok\n", parsed.Sections[0].Header.TableID.Type(), pat.Programs[0].ProgramNumber, pat.Programs[0].ProgramMapID)
	}
	// Output: PAT: program 1 at PMT PID 0x1000, CRC ok
}
