package programmap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProgramMap(t *testing.T) {
	pm := New()
	assert.False(t, pm.ExistsUnlocked(1))
	pm.SetUnlocked(1, 1)
	assert.True(t, pm.ExistsUnlocked(1))
	pm.UnsetUnlocked(1)
	assert.False(t, pm.ExistsUnlocked(1))
}
