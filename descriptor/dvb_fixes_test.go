package descriptor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v3/ts"
)

func parseSingle(t *testing.T, body []byte) Descriptor {
	t.Helper()

	loop := append([]byte{0xf0 | byte(len(body)>>8), byte(len(body))}, body...)
	ds, n, err := Parse(loop)
	require.NoError(t, err)
	require.Equal(t, len(loop), n)
	require.Len(t, ds, 1)
	return ds[0]
}

func TestReencodeDVBDescriptors(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []byte
		want []byte
	}{
		{"S2 keeps the timeslice number",
			[]byte{0x79, 0x02, 0x0c, 0x07}, nil},
		{"S2 flags byte read as v1.15.1 re-encodes to v1.19.1",
			[]byte{0x79, 0x01, 0x1a}, []byte{0x79, 0x01, 0x1e}},
		{"S2 zero flags byte gains its reserved_future_use",
			[]byte{0x79, 0x02, 0x00, 0x07}, []byte{0x79, 0x02, 0x0c, 0x07}},
		{"S2 with scrambling sequence index and input stream identifier",
			[]byte{0x79, 0x05, 0xdc, 0xfc, 0x01, 0x02, 0x03}, nil},
		{"AC-3 reserved_flags are zero",
			[]byte{0x6a, 0x01, 0x09}, []byte{0x6a, 0x01, 0x00}},
		{"DTS-HD reserved_future_use are ones",
			[]byte{0x7f, 0x08, 0x0e, 0x87, 0x05, 0x06, 0xe7, 0x08, 0x03, 0x03}, nil},
		{"video depth range keeps the bytes an overlong range_length hides",
			[]byte{0x7f, 0x05, 0x10, 0x01, 0x02, 0xaa, 0xbb}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := tc.want
			if want == nil {
				want = tc.in
			}
			assert.Equal(t, want, Append(nil, []Descriptor{parseSingle(t, tc.in)}))
		})
	}
}

func TestLocalTimeOffsetRejectsInvalidBCD(t *testing.T) {
	body := []byte{0x65, 0x6e, 0x67, 0x02, 0xab, 0x45, 0xc0, 0x79, 0x12, 0x45, 0x00, 0x01, 0x45}
	in := append([]byte{0x58, byte(len(body))}, body...)

	d, ok := parseSingle(t, in).(*Malformed)
	require.True(t, ok)
	require.ErrorIs(t, d.Err, ts.ErrInvalidData)
	assert.Equal(t, body, d.Raw)
	assert.Equal(t, in, Append(nil, []Descriptor{d}))
}
