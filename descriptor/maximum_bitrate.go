package descriptor

import (
	"fmt"

	"github.com/asticode/go-astikit"
)

// DescriptorMaximumBitrate represents a maximum bitrate descriptor
type DescriptorMaximumBitrate struct {
	Bitrate uint32 // In bytes/second
	Header  DescriptorHeader
}

func newDescriptorMaximumBitrate(i *astikit.BytesIterator, h DescriptorHeader, _ int) (dd Descriptor, err error) {
	// Get next bytes
	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	// Create descriptor
	d := &DescriptorMaximumBitrate{
		Header:  h,
		Bitrate: (uint32(bs[0]&0x3f)<<16 | uint32(bs[1])<<8 | uint32(bs[2])) * 50,
	}
	dd = d
	return
}

func (*DescriptorMaximumBitrate) length() uint8 {
	return 3
}

func (d *DescriptorMaximumBitrate) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.WriteN(uint8(0xff), 2)
	b.WriteN(d.Bitrate/50, 22)

	return written, b.Err()
}
