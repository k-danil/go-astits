package psi

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/descriptor"
	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// TSDT represents a TSDT: the transport stream description table carries
// descriptors that apply to the transport stream as a whole. Its descriptor
// loop is bounded by the section length, like a CAT.
// Chapter: 2.4.4.12 | Link: ISO/IEC 13818-1
type TSDT struct {
	Descriptors []descriptor.Descriptor `json:"_descriptors"`
}

// parseTSDTSection parses a TSDT section
func parseTSDTSection(i *bytesiter.Iterator, offsetSectionsEnd int) (d *TSDT, err error) {
	d = &TSDT{}
	length := offsetSectionsEnd - i.Offset()
	if length <= 0 {
		return
	}
	var n int
	if d.Descriptors, n, err = descriptor.ParseN(i.Bytes(), length); err != nil {
		err = fmt.Errorf("astits: parsing TSDT descriptors failed: %w", err)
		return
	}
	i.Skip(n)
	return
}

func (d *TSDT) CalcSectionLength() int { return descriptor.CalcLength(d.Descriptors) }

// appendSection appends the TSDT body: descriptors bounded by the section length,
// no length prefix. The syntax header and CRC are added by the section framing.
func (d *TSDT) appendSection(dst []byte) []byte {
	return descriptor.Append(dst, d.Descriptors)
}
