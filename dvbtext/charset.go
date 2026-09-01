package dvbtext

import (
	"encoding/binary"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

const (
	selectorISO8859Min = 0x01
	selectorISO8859Max = 0x0b
	selectorDynamic    = 0x10
	selectorUCS2       = 0x11
	selectorKSX1001    = 0x12
	selectorGB2312     = 0x13
	selectorBig5       = 0x14
	selectorUTF8       = 0x15
	selectorCustom     = 0x1f
	selectorNoneMin    = 0x20

	iso8859Offset     = 4
	iso8859Thai       = 11
	encodingTypeIDLen = 1
)

type mode bool

const (
	strict  mode = true
	lenient mode = false
)

// 8859-12 was never published, hence the hole at index 12.
var iso8859Tables = [16]*charmap.Charmap{
	1:  charmap.ISO8859_1,
	2:  charmap.ISO8859_2,
	3:  charmap.ISO8859_3,
	4:  charmap.ISO8859_4,
	5:  charmap.ISO8859_5,
	6:  charmap.ISO8859_6,
	7:  charmap.ISO8859_7,
	8:  charmap.ISO8859_8,
	9:  charmap.ISO8859_9,
	10: charmap.ISO8859_10,
	13: charmap.ISO8859_13,
	14: charmap.ISO8859_14,
	15: charmap.ISO8859_15,
}

func decode(t Text, m mode) (s string, err error) {
	if len(t) == 0 {
		return
	}

	sel, body := t[0], t[1:]
	switch {
	case sel >= selectorNoneMin:
		return decodeLatin(t, m)
	case sel >= selectorISO8859Min && sel <= selectorISO8859Max:
		return decodeISO8859(int(sel)+iso8859Offset, body, m)
	case sel == selectorDynamic:
		return decodeDynamic(body, m)
	case sel == selectorUCS2:
		return decodeUCS2(body, m)
	case sel == selectorKSX1001:
		return decodeMultiByte(korean.EUCKR, body, m)
	case sel == selectorGB2312:
		return decodeMultiByte(simplifiedchinese.GBK, body, m)
	case sel == selectorBig5:
		return decodeMultiByte(traditionalchinese.Big5, body, m)
	case sel == selectorUTF8:
		return decodeUTF8(body, m)
	case sel == selectorCustom:
		if m == strict {
			err = ErrUnsupportedCharset
			return
		}
		if len(body) > 0 {
			body = body[encodingTypeIDLen:]
		}
		return decodeLatin(body, lenient)
	default:
		if m == strict {
			err = ErrUnsupportedCharset
			return
		}
		return decodeLatin(body, lenient)
	}
}

func decodeDynamic(body []byte, m mode) (s string, err error) {
	const partOffset = 2

	if len(body) < partOffset || body[0] != 0 {
		if m == strict {
			err = ErrInvalidText
			return
		}
		return decodeLatin(body, lenient)
	}
	return decodeISO8859(int(body[1]), body[partOffset:], m)
}

func decodeISO8859(part int, body []byte, m mode) (s string, err error) {
	if part == iso8859Thai {
		return decodeThai(body, m)
	}

	var cm *charmap.Charmap
	if part >= 0 && part < len(iso8859Tables) {
		cm = iso8859Tables[part]
	}
	if cm == nil {
		if m == strict {
			err = ErrUnsupportedCharset
			return
		}
		return decodeLatin(body, lenient)
	}
	return decodeCharmap(cm, body, m)
}

func decodeCharmap(cm *charmap.Charmap, body []byte, m mode) (s string, err error) {
	var b strings.Builder
	b.Grow(len(body))

	for _, c := range body {
		if isControl(c) {
			if r, ok := controlRune(c); ok {
				b.WriteRune(r)
			}
			continue
		}

		r := cm.DecodeByte(c)
		if r == utf8.RuneError {
			if m == strict {
				err = ErrInvalidText
				return
			}
			continue
		}
		b.WriteRune(r)
	}

	s = b.String()
	return
}

// x/text carries no ISO/IEC 8859-11 charmap.
func decodeThai(body []byte, m mode) (s string, err error) {
	const (
		thaiOffset  = 0x0d60
		thaiMin     = 0xa1
		thaiGapMin  = 0xdb
		thaiGapMax  = 0xde
		thaiMax     = 0xfb
		nonBreaking = 0xa0
	)

	var b strings.Builder
	b.Grow(len(body))

	for _, c := range body {
		switch {
		case c >= asciiMin && c <= asciiMax:
			b.WriteByte(c)
		case isControl(c):
			if r, ok := controlRune(c); ok {
				b.WriteRune(r)
			}
		case c == nonBreaking:
			b.WriteRune(rune(c))
		case c >= thaiMin && c <= thaiMax && (c < thaiGapMin || c > thaiGapMax):
			b.WriteRune(rune(c) + thaiOffset)
		default:
			if m == strict {
				err = ErrInvalidText
				return
			}
		}
	}

	s = b.String()
	return
}

func decodeUCS2(body []byte, m mode) (s string, err error) {
	const unit = 2

	if len(body)%unit != 0 && m == strict {
		err = ErrInvalidText
		return
	}

	us := make([]uint16, len(body)/unit)
	for i := range us {
		us[i] = binary.BigEndian.Uint16(body[i*unit:])
	}

	s = replaceControlRunes(string(utf16.Decode(us)))
	return
}

func decodeUTF8(body []byte, m mode) (s string, err error) {
	s = string(body)
	if !utf8.ValidString(s) {
		if m == strict {
			s, err = "", ErrInvalidText
			return
		}
		s = strings.ToValidUTF8(s, "")
	}

	s = replaceControlRunes(s)
	return
}

func decodeMultiByte(e encoding.Encoding, body []byte, m mode) (s string, err error) {
	out, decErr := e.NewDecoder().Bytes(body)
	s = replaceControlRunes(string(out))
	if m == strict && (decErr != nil || strings.ContainsRune(s, utf8.RuneError)) {
		s, err = "", ErrInvalidText
	}
	return
}
