package dvb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

// An undefined start time is all ones on the wire and the zero time in Go,
// both ways.
func TestUndefinedTimeRoundTrip(t *testing.T) {
	bs := AppendTime(nil, time.Time{})
	assert.Equal(t, []byte{0xff, 0xff, 0xff, 0xff, 0xff}, bs)

	got, err := ParseTime(bytesiter.New(bs))
	require.NoError(t, err)
	assert.True(t, got.IsZero())
}

func TestParseRejectsNonBCD(t *testing.T) {
	for _, tc := range []struct {
		name  string
		parse func(*bytesiter.Iterator) error
		bs    []byte
	}{
		{"time hours", func(i *bytesiter.Iterator) error { _, err := ParseTime(i); return err },
			[]byte{0xc0, 0x79, 0xa2, 0x45, 0x00}},
		{"time seconds", func(i *bytesiter.Iterator) error { _, err := ParseTime(i); return err },
			[]byte{0xc0, 0x79, 0x12, 0x45, 0x0f}},
		{"duration minutes", func(i *bytesiter.Iterator) error { _, err := ParseDurationMinutes(i); return err },
			[]byte{0x01, 0xab}},
		{"duration seconds", func(i *bytesiter.Iterator) error { _, err := ParseDurationSeconds(i); return err },
			[]byte{0x01, 0x45, 0xfe}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.ErrorIs(t, tc.parse(bytesiter.New(tc.bs)), ErrInvalidBCD)
		})
	}
}

func TestAppendSaturatesOutOfRange(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  []byte
		want []byte
	}{
		{"duration past 99 hours", AppendDurationSeconds(nil, 160*time.Hour), []byte{0x99, 0x59, 0x59}},
		{"negative duration", AppendDurationMinutes(nil, -time.Hour), []byte{0x00, 0x00}},
		{"date before the MJD epoch",
			AppendTime(nil, time.Date(1858, time.November, 16, 1, 2, 3, 0, time.UTC)),
			[]byte{0x00, 0x00, 0x01, 0x02, 0x03}},
		{"date past the MJD range",
			AppendTime(nil, time.Date(2200, time.January, 1, 1, 2, 3, 0, time.UTC)),
			[]byte{0xff, 0xfe, 0x01, 0x02, 0x03}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.got)
		})
	}
}
