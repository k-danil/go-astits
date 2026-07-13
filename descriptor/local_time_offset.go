package descriptor

import (
	"fmt"
	"time"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/dvb"
	"github.com/k-danil/go-astits/v2/internal/util"
)

// LocalTimeOffset represents a local time offset descriptor
// Chapter: 6.2.20 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type LocalTimeOffset struct {
	Header Header                `json:"_header"`
	Items  []LocalTimeOffsetItem `json:"_items"`
}

// LocalTimeOffsetItem represents a local time offset item descriptor
// Chapter: 6.2.20 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type LocalTimeOffsetItem struct {
	LocalTimeOffset         time.Duration `json:"local_time_offset"`
	NextTimeOffset          time.Duration `json:"next_time_offset"`
	TimeOfChange            time.Time     `json:"time_of_change"`
	CountryCode             [3]byte       `json:"country_code"`
	CountryRegionID         uint8         `json:"country_region_id"`
	LocalTimeOffsetPolarity bool          `json:"local_time_offset_polarity"`
}

func newDescriptorLocalTimeOffset(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &LocalTimeOffset{
		Header: h,
		Items:  make([]LocalTimeOffsetItem, (offsetEnd-i.Offset())/13),
	}
	dd = d

	for idx := range d.Items {
		var bs []byte
		if bs, err = i.NextBytesNoCopy(3); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		copy(d.Items[idx].CountryCode[:], bs)

		var b byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		d.Items[idx].CountryRegionID = b >> 2

		d.Items[idx].LocalTimeOffsetPolarity = b&0x1 > 0

		if d.Items[idx].LocalTimeOffset, err = dvb.ParseDurationMinutes(i); err != nil {
			err = fmt.Errorf("astits: parsing DVB durationminutes failed: %w", err)
			return
		}

		if d.Items[idx].TimeOfChange, err = dvb.ParseTime(i); err != nil {
			err = fmt.Errorf("astits: parsing DVB time failed: %w", err)
			return
		}

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
