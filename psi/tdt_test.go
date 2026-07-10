package psi

import (
	"testing"
	"time"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/stretchr/testify/assert"
)

func TestParseTDTSection(t *testing.T) {
	want, _ := time.Parse("2006-01-02 15:04:05", "1993-10-13 12:45:00")
	d, err := parseTDTSection(bytesiter.New([]byte{0xc0, 0x79, 0x12, 0x45, 0x00}))
	assert.NoError(t, err)
	assert.Equal(t, want, d.UTCTime)
}
