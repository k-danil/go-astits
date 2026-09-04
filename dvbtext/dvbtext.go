package dvbtext

import (
	"encoding/json"
	"errors"
	"unicode/utf8"

	"github.com/k-danil/go-astits/v3/internal/errclass"
	"github.com/k-danil/go-astits/v3/ts"
)

var (
	ErrUnsupportedCharset = errors.New("astits: unsupported DVB character table")
	ErrInvalidText        = errclass.New("astits: invalid DVB text", ts.ErrInvalidData)
)

// JSON round trip preserves the decoded text, not the wire bytes.
type Text []byte

func (t Text) Decode() (string, error) { return decode(t, strict) }

func (t Text) String() string {
	s, _ := decode(t, lenient)
	return s
}

func (t Text) MarshalJSON() ([]byte, error) { return json.Marshal(t.String()) }

func (t *Text) UnmarshalJSON(bs []byte) (err error) {
	var s string
	if err = json.Unmarshal(bs, &s); err != nil {
		return
	}
	*t = Encode(s)
	return
}

func Encode(s string) (t Text) {
	var ok bool
	if t, ok = encodeLatin(s); ok {
		return
	}

	t = make(Text, 0, len(s)+1)
	t = append(t, selectorUTF8)
	for _, r := range s {
		if pua, ok := controlPUARune(r); ok {
			r = pua
		}
		t = utf8.AppendRune(t, r)
	}
	return
}
