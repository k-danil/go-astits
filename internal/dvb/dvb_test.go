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
