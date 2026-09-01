// Package dvbtext implements the SI text coding of ETSI EN 300 468 annex A
// and the three-byte ISO 639/3166 codes of the SI descriptors.
package dvbtext

import (
	"encoding/json"
	"errors"

	"github.com/k-danil/go-astits/v2/internal/errclass"
	"github.com/k-danil/go-astits/v2/ts"
)

var (
	ErrUnsupportedCharset = errors.New("astits: unsupported DVB character table")
	ErrInvalidText        = errclass.New("astits: invalid DVB text", ts.ErrInvalidData)
)

// Text marshals to JSON as the decoded string: a round trip preserves the
// text, not the wire bytes.
type Text []byte

func (t Text) Decode() (string, error) { return decode(t, strict) }

// String is Decode without the errors: undecodable bytes are dropped.
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

// Encode picks the default character table when the text fits it, UTF-8 otherwise.
func Encode(s string) (t Text) {
	var ok bool
	if t, ok = encodeLatin(s); ok {
		return
	}

	t = make(Text, 0, len(s)+1)
	t = append(t, selectorUTF8)
	t = append(t, s...)
	return
}
