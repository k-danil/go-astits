package dvbtext

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	for _, tc := range []struct {
		name string
		text Text
		want string
	}{
		{"no selector", Text("Channel 1"), "Channel 1"},
		{"diacritic composes with its base", Text{0xc2, 'e', 0xc1, 'a'}, "éà"},
		{"diacritic before a space stands alone", Text{0xc2, ' '}, "´"},
		{"CRLF control, emphasis dropped", Text{'a', 0x8a, 0x86, 'b', 0x87}, "a\nb"},
		{"control byte inside a one-byte table", Text{0x01, 0xbf, 0x8a, 0xe0}, "П\nр"},
		{"control rune inside a two-byte table", append(Text{0x15}, "a\ue08ab"...), "a\nb"},
		{"iso 8859-11", Text{0x07, 'a', 0xa0, 0xa1, 0xa2}, "a\u00a0กข"},
		{"iso 8859-15", Text{0x0b, 0xa4}, "€"},
		{"dynamic iso 8859-2", Text{0x10, 0x00, 0x02, 0xe1}, "á"},
		{"euc-kr", Text{0x12, 0xb0, 0xa1}, "가"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := tc.text.Decode()
			require.NoError(t, err)
			assert.Equal(t, tc.want, s)
		})
	}
}

func TestDecodeBestEffort(t *testing.T) {
	for _, tc := range []struct {
		name string
		text Text
		want string
	}{
		{"reserved selector reads as the default table", Text{0x08, 'a', 'b'}, "ab"},
		{"encoding_type_id skips its identifier", Text{0x1f, 'X', 'a'}, "a"},
		{"dynamic selector with a reserved second byte", Text{0x10, 0x01, 0x05, 'a'}, "a"},
		{"unassigned position dropped", Text{'a', 0xa6, 'b'}, "ab"},
		{"broken utf-8 dropped", Text{0x15, 0xff, 'a'}, "a"},
		{"odd ucs-2 length truncated", Text{0x11, 0x04, 0x1f, 0x04}, "П"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.text.String())
			_, err := tc.text.Decode()
			assert.Error(t, err)
		})
	}
}

func TestEncode(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		want Text
	}{
		{"composed letter splits into diacritic and base", "café au lait", Text{'c', 'a', 'f', 0xc2, 'e', ' ', 'a', 'u', ' ', 'l', 'a', 'i', 't'}},
		{"spacing mark keeps its space", "´", Text{0xc2, ' '}},
		{"euro takes its DVB position", "€", Text{0xa4}},
		{"newline becomes the CRLF control", "a\nb", Text{'a', 0x8a, 'b'}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Encode(tc.text))
		})
	}
}

func FuzzDecode(f *testing.F) {
	for _, seed := range []Text{
		Text("Channel 1"),
		{0xc2, 'e'},
		{0x01, 0xbf, 0xe0, 0xd8},
		{0x10, 0x00, 0x02, 0xe1},
		{0x11, 0x04, 0x1f},
		{0x15, 0xff},
		{0x1f},
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, bs []byte) {
		text := Text(bs)
		_ = text.String()
		_, _ = text.Decode()
		_ = Encode(text.String())
	})
}
