package ts

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The corrupt-input class contract: package sentinels match both themselves
// and ts.ErrInvalidData.
func TestErrorClasses(t *testing.T) {
	assert.True(t, errors.Is(ErrShortPacket, ErrInvalidData))
	assert.True(t, errors.Is(ErrPacketMustStartWithASyncByte, ErrInvalidData))
	assert.True(t, errors.Is(ErrShortPacket, ErrShortPacket))
	assert.False(t, errors.Is(ErrNoMorePackets, ErrInvalidData))

	var cr ClockReference
	_, err := cr.ParsePCR(nil)
	assert.True(t, errors.Is(err, ErrShortPacket))
	assert.True(t, errors.Is(err, ErrInvalidData))
}
