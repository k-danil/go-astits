package psi

// ST carries stuffing only: the bytes are discarded, not stored.
type ST struct{}

func parseSTSection() (d *ST) {
	return &ST{}
}

func (d *ST) CalcSectionLength() int { return 0 }

func (d *ST) appendSection(dst []byte) []byte { return dst }
