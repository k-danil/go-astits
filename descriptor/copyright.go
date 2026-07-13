package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// Copyright is the MPEG-2 systems copyright_descriptor (ISO/IEC 13818-1).
type Copyright struct {
	AdditionalCopyrightInfo []byte `json:"additional_copyright_info"`
	CopyrightIdentifier     uint32 `json:"copyright_identifier"`
	Header                  Header `json:"_header"`
}

func newDescriptorCopyright(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &Copyright{
		Header:              h,
		CopyrightIdentifier: binary.BigEndian.Uint32(bs),
	}
	dd = d

	if i.Offset() < offsetEnd {
		if d.AdditionalCopyrightInfo, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *Copyright) CalcLength() int {
	return 4 + len(d.AdditionalCopyrightInfo)
}

func (d *Copyright) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, byte(d.CopyrightIdentifier>>24), byte(d.CopyrightIdentifier>>16), byte(d.CopyrightIdentifier>>8), byte(d.CopyrightIdentifier))
	return append(dst, d.AdditionalCopyrightInfo...)
}
