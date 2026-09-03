package psi

import (
	"fmt"
	"time"

	"github.com/k-danil/go-astits/v3/descriptor"
	"github.com/k-danil/go-astits/v3/internal/bytesiter"
	"github.com/k-danil/go-astits/v3/internal/dvb"
)

type TOT struct {
	Descriptors []descriptor.Descriptor `json:"_descriptors"`
	UTCTime     time.Time               `json:"UTC_time"`
}

func parseTOTSection(i *bytesiter.Iterator) (d *TOT, err error) {
	d = &TOT{}

	if d.UTCTime, err = dvb.ParseTime(i); err != nil {
		err = fmt.Errorf("astits: parsing DVB time failed: %w", err)
		return
	}

	var dn int
	if d.Descriptors, dn, err = descriptor.Parse(i.Bytes()); err != nil {
		err = fmt.Errorf("astits: parsing descriptors failed: %w", err)
		return
	}
	i.Skip(dn)
	return
}

func (d *TOT) CalcSectionLength() int {
	return dvbTimeBytesSize + 2 + descriptor.CalcLength(d.Descriptors)
}

func (d *TOT) appendSection(dst []byte) []byte {
	dst = dvb.AppendTime(dst, d.UTCTime)
	return descriptor.AppendWithLength(dst, d.Descriptors)
}
