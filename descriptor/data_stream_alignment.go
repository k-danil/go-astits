package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/internal/bytesiter"
)

// Data stream alignments
// Page: 85 | Chapter:2.6.11 | Link: http://ecee.colorado.edu/~ecen5653/ecen5653/papers/iso13818-1.pdf
const (
	DataStreamAligmentAudioSyncWord          = 0x1
	DataStreamAligmentVideoSliceOrAccessUnit = 0x1
	DataStreamAligmentVideoAccessUnit        = 0x2
	DataStreamAligmentVideoGOPOrSEQ          = 0x3
	DataStreamAligmentVideoSEQ               = 0x4
)

// DataStreamAlignment represents a data stream alignment descriptor
type DataStreamAlignment struct {
	Header Header
	Type   uint8
}

func newDescriptorDataStreamAlignment(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d := &DataStreamAlignment{
		Header: h,
		Type:   b,
	}
	dd = d
	return
}

func (*DataStreamAlignment) CalcLength() int {
	return 1
}

func (d *DataStreamAlignment) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, d.Type)
}
