package descriptor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMalformedBody(t *testing.T) {
	loop := []byte{0xf0, 0x08,
		byte(TagISO639LanguageAndAudioType), 3, 'r', 'u', 's',
		byte(TagStreamIdentifier), 1, 7}
	ds, n, err := Parse(loop)
	require.NoError(t, err)
	assert.Equal(t, len(loop), n)
	require.Len(t, ds, 2)

	m, ok := ds[0].(*Malformed)
	require.True(t, ok)
	assert.Equal(t, Header{Tag: TagISO639LanguageAndAudioType, Length: 3}, m.Header)
	assert.Equal(t, []byte("rus"), m.Raw)
	assert.Error(t, m.Err)
	assert.Equal(t, &StreamIdentifier{Header: Header{Tag: TagStreamIdentifier, Length: 1}, ComponentTag: 7}, ds[1])

	assert.Equal(t, loop[2:], Append(nil, ds))
}
