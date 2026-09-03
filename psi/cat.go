package psi

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/descriptor"
	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type CAT struct {
	Descriptors []descriptor.Descriptor `json:"_descriptors"`
}

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

// No descriptors_loop_length prefix: the section length bounds the loop (same for TSDT).
func (d *CAT) appendSection(dst []byte) []byte {
	return descriptor.Append(dst, d.Descriptors)
}
