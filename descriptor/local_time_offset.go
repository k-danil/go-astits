package descriptor

import (
	"fmt"
	"time"

	"github.com/k-danil/go-astits/internal/bytesiter"
	"github.com/k-danil/go-astits/internal/dvb"
	"github.com/k-danil/go-astits/internal/util"
)

// LocalTimeOffset represents a local time offset descriptor
// Chapter: 6.2.20 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type LocalTimeOffset struct {
	Header Header
	Items  []LocalTimeOffsetItem
}

// LocalTimeOffsetItem represents a local time offset item descriptor
// Chapter: 6.2.20 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type LocalTimeOffsetItem struct {
	LocalTimeOffset         time.Duration
	NextTimeOffset          time.Duration
	TimeOfChange            time.Time
	CountryCode             [3]byte
	CountryRegionID         uint8
	LocalTimeOffsetPolarity bool
}

func newDescriptorLocalTimeOffset(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	// Init
	d := &LocalTimeOffset{
		Header: h,
		Items:  make([]LocalTimeOffsetItem, (offsetEnd-i.Offset())/13),
	}
	dd = d

	// Add items
	for idx := range d.Items {
		// Country code
		var bs []byte
		if bs, err = i.NextBytesNoCopy(3); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		copy(d.Items[idx].CountryCode[:], bs)

		// Get next byte
		var b byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		// Country region ID
		d.Items[idx].CountryRegionID = b >> 2

		// Local time offset polarity
		d.Items[idx].LocalTimeOffsetPolarity = b&0x1 > 0

		// Local time offset
		if d.Items[idx].LocalTimeOffset, err = dvb.ParseDurationMinutes(i); err != nil {
			err = fmt.Errorf("astits: parsing DVB durationminutes failed: %w", err)
			return
		}

		// Time of change
		if d.Items[idx].TimeOfChange, err = dvb.ParseTime(i); err != nil {
			err = fmt.Errorf("astits: parsing DVB time failed: %w", err)
			return
		}

		// Next time offset
		if d.Items[idx].NextTimeOffset, err = dvb.ParseDurationMinutes(i); err != nil {
			err = fmt.Errorf("astits: parsing DVB duration minutes failed: %w", err)
			return
		}
	}
	return
}

func (d *LocalTimeOffset) CalcLength() int {
	return 13 * len(d.Items)
}

func (d *LocalTimeOffset) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	for _, item := range d.Items {
		dst = append(dst, item.CountryCode[:]...)
		dst = append(dst, item.CountryRegionID&0x3f<<2|1<<1|util.B2U(item.LocalTimeOffsetPolarity))
		dst = dvb.AppendDurationMinutes(dst, item.LocalTimeOffset)
		dst = dvb.AppendTime(dst, item.TimeOfChange)
		dst = dvb.AppendDurationMinutes(dst, item.NextTimeOffset)
	}
	return dst
}
