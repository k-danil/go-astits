package psi

// ST represents an ST: the stuffing table is pure filler and carries no
// meaningful payload — it is surfaced only so the SI table set is exhaustive.
// The stuffing bytes themselves are discarded.
// Page: 39 | Chapter: 5.2.7 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type ST struct{}

// parseSTSection parses an ST section. The section body is stuffing bytes the
// framework seeks past, so nothing is read here.
func parseSTSection() (d *ST) {
	return &ST{}
}

func (d *ST) CalcSectionLength() int { return 0 }

// appendSection appends the ST body: the stuffing bytes are meaningless (§5.2.8),
// so a serialized ST is an empty section.
func (d *ST) appendSection(dst []byte) []byte { return dst }
