package psi

import (
	"testing"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDITSection(t *testing.T) {
	for _, tc := range []struct {
		name string
		b    byte
		want bool
	}{
		{"transition set", 0x80, true},
		{"transition clear", 0x7f, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := parseDITSection(bytesiter.New([]byte{tc.b}))
			require.NoError(t, err)
			assert.Equal(t, tc.want, d.TransitionFlag)
		})
	}
}
