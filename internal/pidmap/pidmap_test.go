package pidmap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMap(t *testing.T) {
	var m Map[uint16]
	assert.False(t, m.Has(1))
	m.Set(1, 2)
	assert.True(t, m.Has(1))
	assert.Equal(t, uint16(2), *m.Get(1))
	m.Set(1, 3)
	assert.Equal(t, uint16(3), *m.Get(1))
	m.Remove(1)
	assert.False(t, m.Has(1))
}
