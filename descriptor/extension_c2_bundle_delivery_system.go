package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// ExtensionC2BundleDeliverySystem represents a C2 bundle delivery system
// extension descriptor: the DVB-C2 tuning parameters of every bundled PLP
// required to reassemble a channel-bundled transport stream.
// Chapter: 6.4.5.4 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type ExtensionC2BundleDeliverySystem struct {
	Entries []C2BundleEntry
}

// C2BundleEntry is one bundled PLP of a C2 bundle delivery system descriptor
type C2BundleEntry struct {
	C2SystemTuningFrequency     uint32
	PLPID                       uint8
	DataSliceID                 uint8
	C2SystemTuningFrequencyType uint8
	ActiveOFDMSymbolDuration    uint8
	GuardInterval               uint8
	MasterChannel               bool
}

func newDescriptorExtensionC2BundleDeliverySystem(i *bytesiter.Iterator, offsetEnd int) (d *ExtensionC2BundleDeliverySystem, err error) {
	d = &ExtensionC2BundleDeliverySystem{
		Entries: make([]C2BundleEntry, (offsetEnd-i.Offset())/8),
	}
	for idx := range d.Entries {
		var bs []byte
		if bs, err = i.NextBytesNoCopy(8); err != nil || len(bs) < 8 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		d.Entries[idx].PLPID = bs[0]
		d.Entries[idx].DataSliceID = bs[1]
		d.Entries[idx].C2SystemTuningFrequency = binary.BigEndian.Uint32(bs[2:6])
		d.Entries[idx].C2SystemTuningFrequencyType = bs[6] >> 6 & 0x03
		d.Entries[idx].ActiveOFDMSymbolDuration = bs[6] >> 3 & 0x07
		d.Entries[idx].GuardInterval = bs[6] & 0x07
		d.Entries[idx].MasterChannel = bs[7]&0x80 > 0
	}
	return
}

func (d *ExtensionC2BundleDeliverySystem) CalcLength() int {
	return 8 * len(d.Entries)
}

func (d *ExtensionC2BundleDeliverySystem) Append(dst []byte) []byte {
	for idx := range d.Entries {
		e := &d.Entries[idx]
		dst = append(dst, e.PLPID, e.DataSliceID,
			byte(e.C2SystemTuningFrequency>>24), byte(e.C2SystemTuningFrequency>>16),
			byte(e.C2SystemTuningFrequency>>8), byte(e.C2SystemTuningFrequency),
			e.C2SystemTuningFrequencyType&0x03<<6|e.ActiveOFDMSymbolDuration&0x07<<3|e.GuardInterval&0x07)
		var master byte
		if e.MasterChannel {
			master = 0x80
		}
		dst = append(dst, master)
	}
	return dst
}
