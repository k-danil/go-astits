package descriptor

import "github.com/asticode/go-astikit"

func newDescriptorUserDefined(i *astikit.BytesIterator, h DescriptorHeader, _ int) (dd Descriptor, err error) {
	d := &DescriptorUserDefined{
		Header: h,
	}
	dd = d
	d.Data, err = i.NextBytes(int(h.Length))
	return
}

type DescriptorUserDefined struct {
	Header DescriptorHeader
	Data   []byte
}

func (d *DescriptorUserDefined) length() uint8 {
	return uint8(len(d.Data))
}

func (d *DescriptorUserDefined) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.Data)

	return written, b.Err()
}
