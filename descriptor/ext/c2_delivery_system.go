package ext

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// C2DeliverySystem represents a C2 delivery system extension
// descriptor: the DVB-C2 tuning parameters mapping a transport stream to a data
// PLP.
// Chapter: 6.4.5.1 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type C2DeliverySystem struct {
	C2SystemTuningFrequency     uint32 `json:"C2_System_tuning_frequency"`
	PLPID                       uint8  `json:"plp_id"`
	DataSliceID                 uint8  `json:"data_slice_id"`
	C2SystemTuningFrequencyType uint8  `json:"C2_System_tuning_frequency_type"`
	ActiveOFDMSymbolDuration    uint8  `json:"active_OFDM_symbol_duration"`
	GuardInterval               uint8  `json:"guard_interval"`
}

func parseC2DeliverySystem(i *bytesiter.Iterator, _ int) (d *C2DeliverySystem, err error) {
	d = &C2DeliverySystem{}

	var bs []byte
	if bs, err = i.NextBytesNoCopy(7); err != nil || len(bs) < 7 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d.PLPID = bs[0]
	d.DataSliceID = bs[1]
	d.C2SystemTuningFrequency = binary.BigEndian.Uint32(bs[2:6])
	d.C2SystemTuningFrequencyType = bs[6] >> 6 & 0x03
	d.ActiveOFDMSymbolDuration = bs[6] >> 3 & 0x07
	d.GuardInterval = bs[6] & 0x07
	return
}

func (d *C2DeliverySystem) CalcLength() int {
	return 7
}

func (d *C2DeliverySystem) Append(dst []byte) []byte {
	dst = append(dst, d.PLPID, d.DataSliceID,
		byte(d.C2SystemTuningFrequency>>24), byte(d.C2SystemTuningFrequency>>16),
		byte(d.C2SystemTuningFrequency>>8), byte(d.C2SystemTuningFrequency))
	return append(dst, d.C2SystemTuningFrequencyType&0x03<<6|
		d.ActiveOFDMSymbolDuration&0x07<<3|d.GuardInterval&0x07)
}
