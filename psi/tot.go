package psi

import (
	"fmt"
	"time"

	"github.com/k-danil/go-astits/v2/descriptor"
	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/dvb"
)

// TOT represents a TOT data
// Page: 39 | Chapter: 5.2.6 | Link: https://www.dvb.org/resources/public/standards/a38_dvb-si_specification.pdf
// (barbashov) the link above can be broken, alternative: https://dvb.org/wp-content/uploads/2019/12/a038_tm1217r37_en300468v1_17_1_-_rev-134_-_si_specification.pdf
type TOT struct {
	Descriptors []descriptor.Descriptor
	UTCTime     time.Time
}

// parseTOTSection parses a TOT section
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
	// UTC_time + reserved/descriptors_loop_length prefix + descriptors
	return dvbTimeBytesSize + 2 + descriptor.CalcLength(d.Descriptors)
}

func (d *TOT) appendSection(dst []byte) []byte {
	dst = dvb.AppendTime(dst, d.UTCTime)
	return descriptor.AppendWithLength(dst, d.Descriptors)
}
