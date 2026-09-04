package ext

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type C2BundleDeliverySystem struct {
	Entries []C2BundleEntry `json:"_entries"`
}

type C2BundleEntry struct {
	C2SystemTuningFrequency     uint32 `json:"C2_System_tuning_frequency"`
	PLPID                       uint8  `json:"plp_id"`
	DataSliceID                 uint8  `json:"data_slice_id"`
	C2SystemTuningFrequencyType uint8  `json:"C2_System_tuning_frequency_type"`
	ActiveOFDMSymbolDuration    uint8  `json:"active_OFDM_symbol_duration"`
	GuardInterval               uint8  `json:"guard_interval"`
	PrimaryChannel              bool   `json:"primary_channel"`
}

const c2BundleEntrySize = 8

func parseC2BundleDeliverySystem(i *bytesiter.Iterator, offsetEnd int) (d *C2BundleDeliverySystem, err error) {
	d = &C2BundleDeliverySystem{
		Entries: make([]C2BundleEntry, (offsetEnd-i.Offset())/c2BundleEntrySize),
	}
	for idx := range d.Entries {
		var bs []byte
		if bs, err = i.NextBytesNoCopy(c2BundleEntrySize); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		d.Entries[idx].PLPID = bs[0]
		d.Entries[idx].DataSliceID = bs[1]
		d.Entries[idx].C2SystemTuningFrequency = binary.BigEndian.Uint32(bs[2:6])
		d.Entries[idx].C2SystemTuningFrequencyType = bs[6] >> 6 & 0x03
		d.Entries[idx].ActiveOFDMSymbolDuration = bs[6] >> 3 & 0x07
		d.Entries[idx].GuardInterval = bs[6] & 0x07
		d.Entries[idx].PrimaryChannel = bs[7]&0x80 > 0
	}

	err = rejectTrailingBytes(i, offsetEnd)
	return
}

func (d *C2BundleDeliverySystem) CalcLength() int {
	return c2BundleEntrySize * len(d.Entries)
}

func (d *C2BundleDeliverySystem) Append(dst []byte) []byte {
	for idx := range d.Entries {
		e := &d.Entries[idx]
		dst = append(dst, e.PLPID, e.DataSliceID,
			byte(e.C2SystemTuningFrequency>>24), byte(e.C2SystemTuningFrequency>>16),
			byte(e.C2SystemTuningFrequency>>8), byte(e.C2SystemTuningFrequency),
			e.C2SystemTuningFrequencyType&0x03<<6|e.ActiveOFDMSymbolDuration&0x07<<3|e.GuardInterval&0x07)
		var primary byte
		if e.PrimaryChannel {
			primary = 0x80
		}
		dst = append(dst, primary)
	}
	return dst
}
