package util

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const (
	hexPrefix           = "0x"
	hexSuffixOpen       = "(0x"
	hexSuffixClose      = ")"
	enumBitSize         = 8
	quote          byte = '"'
)

func UnmarshalEnum[T ~uint8](b []byte, names map[T]string) (v T, err error) {
	if len(b) > 0 && b[0] != quote {
		var n uint8
		if err = json.Unmarshal(b, &n); err != nil {
			return
		}
		v = T(n)
		return
	}
	var s string
	if err = json.Unmarshal(b, &s); err != nil {
		return
	}
	v, err = EnumFromString(s, names)
	return
}

func EnumFromString[T ~uint8](s string, names map[T]string) (v T, err error) {
	for k, name := range names {
		if name == s {
			v = k
			return
		}
	}
	hex := ""
	if strings.HasPrefix(s, hexPrefix) {
		hex = s[len(hexPrefix):]
	} else if i := strings.LastIndex(s, hexSuffixOpen); i >= 0 && strings.HasSuffix(s, hexSuffixClose) {
		hex = s[i+len(hexSuffixOpen) : len(s)-len(hexSuffixClose)]
	}
	if hex == "" {
		err = fmt.Errorf("astits: unknown enum name %q", s)
		return
	}
	var n uint64
	if n, err = strconv.ParseUint(hex, 16, enumBitSize); err != nil {
		err = fmt.Errorf("astits: parsing enum hex %q failed: %w", s, err)
		return
	}
	v = T(n)
	return
}
