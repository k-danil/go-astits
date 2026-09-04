package ext

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
	"github.com/k-danil/go-astits/v3/ts"
)

func TestT2DeliverySystemLengthMatchesAppend(t *testing.T) {
	for _, tc := range []struct {
		name  string
		freqs []uint32
	}{
		{"no centre frequency", nil},
		{"one centre frequency", []uint32{1}},
		{"two centre frequencies", []uint32{1, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &T2DeliverySystem{HasExtension: true,
				Cells: []T2Cell{{CellID: 7, CentreFrequencies: tc.freqs}}}

			assert.Equal(t, len(d.Append(nil)), d.CalcLength())
		})
	}
}

func TestVideoDepthRangeFollowsRangeLength(t *testing.T) {
	body := []byte{0x00, 0x03, 0x12, 0x34, 0x56, 0x02, 0x01, 0xbb}

	d, err := parseVideoDepthRange(bytesiter.New(body), len(body))
	require.NoError(t, err)
	require.Len(t, d.Ranges, 2)
	assert.EqualValues(t, 0x123, d.Ranges[0].VideoMaxDisparityHint)
	assert.EqualValues(t, 0x456, d.Ranges[0].VideoMinDisparityHint)
	assert.EqualValues(t, 0x02, d.Ranges[1].RangeType)
	assert.Equal(t, []byte{0xbb}, d.Ranges[1].RangeSelector)
}

func TestVideoDepthRangeRejectsMismatchedRangeLength(t *testing.T) {
	for _, tc := range []struct {
		name    string
		body    []byte
		wantErr string
	}{
		{"declared shorter than the hint", []byte{0x00, 0x00, 0x12, 0x34, 0x56}, "overruns range_length"},
		{"declared longer than the fields it carries", []byte{0x01, 0x02, 0xaa, 0xbb}, "bytes left inside range_length"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseVideoDepthRange(bytesiter.New(tc.body), len(tc.body))
			require.ErrorIs(t, err, ts.ErrInvalidData)
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}
