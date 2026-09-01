package dvbtext

import (
	"strings"
	"sync"

	"golang.org/x/text/unicode/norm"
)

const (
	asciiMin = 0x20
	asciiMax = 0x7e
	highMin  = 0xa0
	markMin  = 0xc0
)

// Character code table 00 (figure A.1): ISO/IEC 6937 with the Euro at 0xa4,
// the only position DVB adds. Values follow the figure, which parts ways with
// some ISO 6937 mappings at 0xd0 and 0xe2 — do not "fix" those two.
// 0xc0..0xcf stay zero: those positions hold the non-spacing marks below.
var latinHigh = [96]rune{
	0x00A0, 0x00A1, 0x00A2, 0x00A3, 0x20AC, 0x00A5, 0, 0x00A7, 0x00A4, 0x2018, 0x201C, 0x00AB, 0x2190, 0x2191, 0x2192, 0x2193,
	0x00B0, 0x00B1, 0x00B2, 0x00B3, 0x00D7, 0x00B5, 0x00B6, 0x00B7, 0x00F7, 0x2019, 0x201D, 0x00BB, 0x00BC, 0x00BD, 0x00BE, 0x00BF,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0x2015, 0x00B9, 0x00AE, 0x00A9, 0x2122, 0x266A, 0x00AC, 0x00A6, 0, 0, 0, 0, 0x215B, 0x215C, 0x215D, 0x215E,
	0x2126, 0x00C6, 0x0110, 0x00AA, 0x0126, 0, 0x0132, 0x013F, 0x0141, 0x00D8, 0x0152, 0x00BA, 0x00DE, 0x0166, 0x014A, 0x0149,
	0x0138, 0x00E6, 0x0111, 0x00F0, 0x0127, 0x0131, 0x0133, 0x0140, 0x0142, 0x00F8, 0x0153, 0x00DF, 0x00FE, 0x0167, 0x014B, 0x00AD,
}

var latinMarks = [16]rune{
	0, 0x0300, 0x0301, 0x0302, 0x0303, 0x0304, 0x0306, 0x0307,
	0x0308, 0, 0x030A, 0x0327, 0, 0x030B, 0x0328, 0x030C,
}

var latinSpacingMarks = [16]rune{
	0, 0, 0x00B4, 0, 0, 0x00AF, 0x02D8, 0x02D9,
	0x00A8, 0, 0x02DA, 0x00B8, 0, 0x02DD, 0x02DB, 0x02C7,
}

func latinMark(c byte) rune {
	if c < markMin || int(c)-markMin >= len(latinMarks) {
		return 0
	}
	return latinMarks[c-markMin]
}

func decodeLatin(body []byte, m mode) (s string, err error) {
	var b strings.Builder
	b.Grow(len(body))

	for i := 0; i < len(body); i++ {
		c := body[i]
		mark := latinMark(c)
		switch {
		case mark != 0:
			var consumed int
			if consumed, err = writeComposed(&b, mark, latinSpacingMarks[c-markMin], body[i+1:], m); err != nil {
				return
			}
			i += consumed
		case c >= asciiMin && c <= asciiMax:
			b.WriteByte(c)
		case isControl(c):
			if r, ok := controlRune(c); ok {
				b.WriteRune(r)
			}
		case c >= highMin && latinHigh[c-highMin] != 0:
			b.WriteRune(latinHigh[c-highMin])
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

func writeComposed(b *strings.Builder, mark, spacing rune, rest []byte, m mode) (consumed int, err error) {
	if len(rest) == 0 {
		if m == strict {
			err = ErrInvalidText
		}
		return
	}

	base := rest[0]
	switch {
	case base == ' ':
		consumed = 1
		if spacing != 0 {
			b.WriteRune(spacing)
			return
		}
		b.WriteByte(base)
		b.WriteRune(mark)
	case base > ' ' && base <= asciiMax:
		consumed = 1
		b.WriteString(norm.NFC.String(string([]rune{rune(base), mark})))
	default:
		if m == strict {
			err = ErrInvalidText
		}
	}
	return
}

var latinReverse = sync.OnceValue(func() map[rune]uint16 {
	m := make(map[rune]uint16, len(latinHigh)+len(latinSpacingMarks))
	for i, r := range latinHigh {
		if r != 0 {
			m[r] = uint16(highMin + i)
		}
	}
	for i, r := range latinSpacingMarks {
		if r != 0 {
			m[r] = markBase(byte(markMin+i), ' ')
		}
	}
	return m
})

func encodeLatin(s string) (t Text, ok bool) {
	rev := latinReverse()
	t = make(Text, 0, len(s))

	for _, r := range s {
		switch {
		case r == '\n':
			t = append(t, controlCRLF)
		case r >= asciiMin && r <= asciiMax:
			t = append(t, byte(r))
		default:
			code, found := rev[r]
			if !found {
				if code, found = decomposeLatin(r); !found {
					t = nil
					return
				}
			}
			if code > 0xff {
				t = append(t, byte(code>>8))
			}
			t = append(t, byte(code))
		}
	}

	ok = true
	return
}

func markBase(mark, base byte) uint16 { return uint16(mark)<<8 | uint16(base) }

func decomposeLatin(r rune) (code uint16, ok bool) {
	const composed = 2

	rs := []rune(norm.NFD.String(string(r)))
	if len(rs) != composed || rs[0] < asciiMin || rs[0] > asciiMax {
		return
	}

	for i, mark := range latinMarks {
		if mark != 0 && mark == rs[1] {
			code, ok = markBase(byte(markMin+i), byte(rs[0])), true
			return
		}
	}
	return
}
