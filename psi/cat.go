package psi

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/descriptor"
	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// CAT represents a CAT: the conditional access table lists, via CA descriptors,
// the CA systems and their EMM PIDs for the whole transport stream.
// Chapter: 2.4.4.6 | Link: ISO/IEC 13818-1
type CAT struct {
	Descriptors []descriptor.Descriptor
}

// parseCATSection parses a CAT section. Its descriptor loop is bounded by the
// section length rather than a leading loop-length, so it uses descriptor.ParseN.
func parseCATSection(i *bytesiter.Iterator, offsetSectionsEnd int) (d *CAT, err error) {
	d = &CAT{}
	length := offsetSectionsEnd - i.Offset()
	if length <= 0 {
		return
	}
	var n int
	if d.Descriptors, n, err = descriptor.ParseN(i.Bytes(), length); err != nil {
		err = fmt.Errorf("astits: parsing CAT descriptors failed: %w", err)
		return
	}
	i.Skip(n)
	return
}

func (d *CAT) CalcSectionLength() int { return descriptor.CalcLength(d.Descriptors) }

// appendSection appends the CAT body: descriptors bounded by the section length,
// no loop-length prefix. The syntax header and CRC are added by the framing.
func (d *CAT) appendSection(dst []byte) []byte {
	return descriptor.Append(dst, d.Descriptors)
}
