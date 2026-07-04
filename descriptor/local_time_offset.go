package descriptor

import (
	"fmt"
	"time"

	"github.com/asticode/go-astikit"

	"github.com/k-danil/go-astits/internal/dvb"
)

// DescriptorLocalTimeOffset represents a local time offset descriptor
// Chapter: 6.2.20 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorLocalTimeOffset struct {
	Header DescriptorHeader
	Items  []DescriptorLocalTimeOffsetItem
}

// DescriptorLocalTimeOffsetItem represents a local time offset item descriptor
// Chapter: 6.2.20 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorLocalTimeOffsetItem struct {
	LocalTimeOffset         time.Duration
	NextTimeOffset          time.Duration
	TimeOfChange            time.Time
	CountryCode             [3]byte
	CountryRegionID         uint8
	LocalTimeOffsetPolarity bool
}

func newDescriptorLocalTimeOffset(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (dd Descriptor, err error) {
	// Init
	d := &DescriptorLocalTimeOffset{
		Header: h,
		Items:  make([]DescriptorLocalTimeOffsetItem, (offsetEnd-i.Offset())/13),
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

func (d *DescriptorLocalTimeOffset) length() uint8 {
	return uint8(13 * len(d.Items))
}

func (d *DescriptorLocalTimeOffset) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	for _, item := range d.Items {
		b.Write(item.CountryCode[:])

		b.WriteN(item.CountryRegionID, 6)
		b.WriteN(uint8(0xff), 1)
		b.Write(item.LocalTimeOffsetPolarity)

		if _, err := dvb.WriteDurationMinutes(w, item.LocalTimeOffset); err != nil {
			return 0, err
		}
		if _, err := dvb.WriteTime(w, item.TimeOfChange); err != nil {
			return 0, err
		}
		if _, err := dvb.WriteDurationMinutes(w, item.NextTimeOffset); err != nil {
			return 0, err
		}
	}

	return written, b.Err()
}
