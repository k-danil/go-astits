package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

// TerrestrialDeliverySystem holds CentreFrequency in multiples of 10 Hz.
type TerrestrialDeliverySystem struct {
	Header               Header `json:"_header"`
	CentreFrequency      uint32 `json:"centre_frequency"`
	Bandwidth            uint8  `json:"bandwidth"`
	Constellation        uint8  `json:"constellation"`
	HierarchyInformation uint8  `json:"hierarchy_information"`
	CodeRateHPStream     uint8  `json:"code_rate-HP_stream"`
	CodeRateLPStream     uint8  `json:"code_rate-LP_stream"`
	GuardInterval        uint8  `json:"guard_interval"`
	TransmissionMode     uint8  `json:"transmission_mode"`
	Priority             bool   `json:"priority"`
	TimeSlicingIndicator bool   `json:"time_slicing_indicator"`
	MPEFECIndicator      bool   `json:"MPE-FEC_indicator"`
	OtherFrequencyFlag   bool   `json:"other_frequency_flag"`
}

func newDescriptorTerrestrialDeliverySystem(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &TerrestrialDeliverySystem{
		Header: h,
	}
	dd = d

	var bs []byte
	if bs, err = i.NextBytesNoCopy(11); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d.CentreFrequency = binary.BigEndian.Uint32(bs[0:4])
	d.Bandwidth = bs[4] >> 5 & 0x07
	d.Priority = bs[4]&0x10 > 0
	d.TimeSlicingIndicator = bs[4]&0x08 > 0
	d.MPEFECIndicator = bs[4]&0x04 > 0
	d.Constellation = bs[5] >> 6 & 0x03
	d.HierarchyInformation = bs[5] >> 3 & 0x07
	d.CodeRateHPStream = bs[5] & 0x07
	d.CodeRateLPStream = bs[6] >> 5 & 0x07
	d.GuardInterval = bs[6] >> 3 & 0x03
	d.TransmissionMode = bs[6] >> 1 & 0x03
	d.OtherFrequencyFlag = bs[6]&0x01 > 0

	err = rejectTrailingBytes(i, offsetEnd)
	return
}

func (d *TerrestrialDeliverySystem) CalcLength() int {
	return 11
}

func (d *TerrestrialDeliverySystem) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	dst = append(dst,
		byte(d.CentreFrequency>>24), byte(d.CentreFrequency>>16),
		byte(d.CentreFrequency>>8), byte(d.CentreFrequency))

	b4 := d.Bandwidth&0x07<<5 | 0x03
	if d.Priority {
		b4 |= 0x10
	}
	if d.TimeSlicingIndicator {
		b4 |= 0x08
	}
	if d.MPEFECIndicator {
		b4 |= 0x04
	}
	b5 := d.Constellation&0x03<<6 | d.HierarchyInformation&0x07<<3 | d.CodeRateHPStream&0x07
	b6 := d.CodeRateLPStream&0x07<<5 | d.GuardInterval&0x03<<3 | d.TransmissionMode&0x03<<1
	if d.OtherFrequencyFlag {
		b6 |= 0x01
	}
	dst = append(dst, b4, b5, b6)
	return append(dst, 0xff, 0xff, 0xff, 0xff)
}
