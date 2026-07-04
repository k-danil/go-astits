package descriptor

import "github.com/k-danil/go-astits/internal/bytesiter"

func newDescriptorUserDefined(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	d := &UserDefined{
		Header: h,
	}
	dd = d
	d.Data, err = i.NextBytes(int(h.Length))
	return
}

type UserDefined struct {
	Header Header
	Data   []byte
}

func (d *UserDefined) CalcLength() int {
	return len(d.Data)
}

func (d *UserDefined) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, d.Data...)
}
