package ts

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPIDSet(t *testing.T) {
	// Cover the low/mid/high ends of the 13-bit range and a word boundary.
	var s PIDSet
	for _, pid := range []uint16{0, 63, 64, 0x100, 0x1000, 8191} {
		assert.False(t, s.Has(pid), "empty set has %d", pid)
		s.Add(pid)
		assert.True(t, s.Has(pid), "after Add %d", pid)
	}
	assert.False(t, s.Has(1), "unadded pid")
	assert.False(t, s.Has(65), "unadded pid on a set word")

	s.Remove(64)
	assert.False(t, s.Has(64), "after Remove")
	assert.True(t, s.Has(63), "Remove leaves the neighbour bit")

	assert.Equal(t, NewPIDSet(1, 2, 3), func() PIDSet { var x PIDSet; x.Add(1); x.Add(2); x.Add(3); return x }())

	// Out-of-range values fold into the 13-bit space instead of indexing past
	// the set: 8192 collapses to PID 0, no panic.
	var oob PIDSet
	oob.Add(8192)
	assert.True(t, oob.Has(0), "8192 folds to PID 0")
	assert.True(t, oob.Has(8192), "Has folds the same way")

	s.Clear()
	for pid := 0; pid < 8192; pid++ {
		assert.False(t, s.Has(uint16(pid)))
	}
}
