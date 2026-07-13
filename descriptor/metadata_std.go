package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// MetadataSTD is the MPEG-2 systems metadata_STD_descriptor (ISO/IEC 13818-1).
type MetadataSTD struct {
	InputLeakRate  uint32 `json:"metadata_input_leak_rate"`  // 400 bits/s
	BufferSize     uint32 `json:"metadata_buffer_size"`      // 1024 bytes
	OutputLeakRate uint32 `json:"metadata_output_leak_rate"` // 400 bits/s
	Header         Header `json:"_header"`
}

func newDescriptorMetadataSTD(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(9); err != nil || len(bs) < 9 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &MetadataSTD{
		Header:         h,
		InputLeakRate:  uint32(bs[0]&0x3f)<<16 | uint32(bs[1])<<8 | uint32(bs[2]),
		BufferSize:     uint32(bs[3]&0x3f)<<16 | uint32(bs[4])<<8 | uint32(bs[5]),
		OutputLeakRate: uint32(bs[6]&0x3f)<<16 | uint32(bs[7])<<8 | uint32(bs[8]),
	}
	dd = d
	return
}

func (*MetadataSTD) CalcLength() int {
	return 9
}

func (d *MetadataSTD) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, 0xc0|byte(d.InputLeakRate>>16)&0x3f, byte(d.InputLeakRate>>8), byte(d.InputLeakRate))
	dst = append(dst, 0xc0|byte(d.BufferSize>>16)&0x3f, byte(d.BufferSize>>8), byte(d.BufferSize))
	return append(dst, 0xc0|byte(d.OutputLeakRate>>16)&0x3f, byte(d.OutputLeakRate>>8), byte(d.OutputLeakRate))
}
