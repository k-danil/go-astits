package util

func B2U(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
