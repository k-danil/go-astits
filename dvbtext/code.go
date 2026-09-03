package dvbtext

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidCode = errors.New("astits: invalid ISO 639/3166 code")

const (
	codePrintableMin = 0x20
	codePrintableMax = 0x7e
	codeHexPrefix    = "0x"
)

type Code [3]byte

func (c Code) String() (s string) {
	printable := 0
	for printable < len(c) && c[printable] >= codePrintableMin && c[printable] <= codePrintableMax {
		printable++
	}

	s = string(c[:printable])
	// A code literally spelling "0x…" must go out as hex, or ParseCode reads it back as hex digits.
	if printable == 0 || !isZero(c[printable:]) || strings.HasPrefix(s, codeHexPrefix) {
		s = fmt.Sprintf("%s%02x%02x%02x", codeHexPrefix, c[0], c[1], c[2])
	}
	return
}

func (c Code) MarshalJSON() ([]byte, error) { return json.Marshal(c.String()) }

func (c *Code) UnmarshalJSON(bs []byte) (err error) {
	var s string
	if err = json.Unmarshal(bs, &s); err != nil {
		return
	}
	*c, err = ParseCode(s)
	return
}

func ParseCode(s string) (c Code, err error) {
	if digits, ok := strings.CutPrefix(s, codeHexPrefix); ok {
		var bs []byte
		if bs, err = hex.DecodeString(digits); err != nil || len(bs) != len(c) {
			err = ErrInvalidCode
			return
		}
		copy(c[:], bs)
		return
	}

	if len(s) > len(c) {
		err = ErrInvalidCode
		return
	}
	for i := range len(s) {
		if s[i] < codePrintableMin || s[i] > codePrintableMax {
			c, err = Code{}, ErrInvalidCode
			return
		}
		c[i] = s[i]
	}
	return
}

func isZero(bs []byte) bool {
	for _, b := range bs {
		if b != 0 {
			return false
		}
	}
	return true
}
