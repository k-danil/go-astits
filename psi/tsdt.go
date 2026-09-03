package psi

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/descriptor"
	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type TSDT struct {
	Descriptors []descriptor.Descriptor `json:"_descriptors"`
}

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

func (d *TSDT) appendSection(dst []byte) []byte {
	return descriptor.Append(dst, d.Descriptors)
}
