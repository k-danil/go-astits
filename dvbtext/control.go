package dvbtext

import "strings"

// Annex A A.1 reserves all of 0x80..0x9f for control functions, not just the CR/LF we map.
const (
	controlMin     = 0x80
	controlMax     = 0x9f
	controlCRLF    = 0x8a
	controlPUABase = 0xe000
)

func isControl(c byte) bool { return c >= controlMin && c <= controlMax }

func controlRune(c byte) (r rune, ok bool) {
	if c == controlCRLF {
		r, ok = '\n', true
	}
	return
}

// Table A.2: a control code inside a multi-byte table travels as the UTF-8 of its private-use rune, never as the raw byte.
func controlPUARune(r rune) (pua rune, ok bool) {
	if r == '\n' {
		pua, ok = controlPUABase+controlCRLF, true
	}
	return
}

func replaceControlRunes(s string) string {
	if !strings.ContainsFunc(s, isControlRune) {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if isControlRune(r) {
			if rr, ok := controlRune(byte(r - controlPUABase)); ok {
				b.WriteRune(rr)
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isControlRune(r rune) bool {
	return r >= controlPUABase+controlMin && r <= controlPUABase+controlMax
}
