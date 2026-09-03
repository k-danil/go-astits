package descriptor

// Malformed carries a descriptor body the parser rejected, kept verbatim so the surrounding loop stays parseable.
type Malformed struct {
	Err    error  `json:"-"`
	Header Header `json:"_header"`
	Raw    []byte `json:"_raw"`
}

func (d *Malformed) Tag() Tag { return d.Header.Tag }

func (d *Malformed) CalcLength() int {
	return len(d.Raw)
}

func (d *Malformed) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, d.Raw...)
}
