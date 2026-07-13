package descriptor

import "github.com/k-danil/go-astits/v2/internal/bytesiter"

func newDescriptorUserDefined(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	d := &UserDefined{
		Header: h,
	}
	dd = d
	d.Data, err = i.NextBytes(int(h.Length))
	return
}

// UserDefined holds the raw body of a descriptor in the user-defined tag
// range (0x80 and above), whose meaning is private to the stream.
type UserDefined struct {
	Header Header `json:"_header"`
	Data   []byte `json:"_data"`
}

func (d *UserDefined) CalcLength() int {
	return len(d.Data)
}

func (d *UserDefined) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, d.Data...)
}
