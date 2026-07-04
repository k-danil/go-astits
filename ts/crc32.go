package ts

const CRC32Polynomial = uint32(0xffffffff)

func ComputeCRC32(bs []byte) uint32 {
	return UpdateCRC32(CRC32Polynomial, bs)
}

// Based on VLC implementation using a static CRC table (1kb additional memory on start, without
// reallocations): https://github.com/videolan/vlc/blob/master/modules/mux/mpeg/ps.c
func UpdateCRC32(crc32 uint32, bs []byte) uint32 {
	for _, b := range bs {
		crc32 = (crc32 << 8) ^ tableCRC32[uint8(crc32>>24)^b]
	}
	return crc32
}
