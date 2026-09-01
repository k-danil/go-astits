package dvbtext

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCode(t *testing.T) {
	for _, tc := range []struct {
		name string
		code Code
		want string
	}{
		{"three letters", Code{'e', 'n', 'g'}, "eng"},
		{"two letters padded with NUL", Code{'e', 'n', 0}, "en"},
		{"trailing space is not padding", Code{'e', 'n', ' '}, "en "},
		{"unset code", Code{}, "0x000000"},
		{"NUL between letters", Code{'e', 0, 'g'}, "0x650067"},
		{"printable code that reads as hex", Code{'0', 'x', '1'}, "0x307831"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			js, err := json.Marshal(tc.code)
			require.NoError(t, err)
			assert.Equal(t, `"`+tc.want+`"`, string(js))

			var back Code
			require.NoError(t, json.Unmarshal(js, &back))
			assert.Equal(t, tc.code, back)
		})
	}
}

func TestParseCodeRejects(t *testing.T) {
	for _, s := range []string{"engl", "0x1234", "0x000000zz", "e\x01g"} {
		_, err := ParseCode(s)
		assert.ErrorIs(t, err, ErrInvalidCode, s)
	}
}
