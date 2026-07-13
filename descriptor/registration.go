package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// Registration represents a registration descriptor
// Page: 84 | http://ecee.colorado.edu/~ecen5653/ecen5653/papers/iso13818-1.pdf
type Registration struct {
	AdditionalIdentificationInfo []byte `json:"additional_identification_info"`
	FormatIdentifier             uint32 `json:"format_identifier"`
	Header                       Header `json:"_header"`
}

func newDescriptorRegistration(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &Registration{
		Header:           h,
		FormatIdentifier: binary.BigEndian.Uint32(bs),
	}
	dd = d

	if i.Offset() < offsetEnd {
		if d.AdditionalIdentificationInfo, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *Registration) CalcLength() int {
	return 4 + len(d.AdditionalIdentificationInfo)
}

func (d *Registration) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, byte(d.FormatIdentifier>>24), byte(d.FormatIdentifier>>16), byte(d.FormatIdentifier>>8), byte(d.FormatIdentifier))
	return append(dst, d.AdditionalIdentificationInfo...)
}
