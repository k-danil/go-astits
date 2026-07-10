package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// TerrestrialDeliverySystem represents a terrestrial delivery system
// descriptor. CentreFrequency is in multiples of 10 Hz; the remaining fields
// are the DVB-T signalling codes (bandwidth, constellation, hierarchy, code
// rates, guard interval, transmission mode).
// Chapter: 6.2.13.4 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type TerrestrialDeliverySystem struct {
	Header               Header
	CentreFrequency      uint32
	Bandwidth            uint8
	Constellation        uint8
	HierarchyInformation uint8
	CodeRateHPStream     uint8
	CodeRateLPStream     uint8
	GuardInterval        uint8
	TransmissionMode     uint8
	Priority             bool
	TimeSlicingIndicator bool
	MPEFECIndicator      bool
	OtherFrequencyFlag   bool
}

func newDescriptorTerrestrialDeliverySystem(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	d := &TerrestrialDeliverySystem{
		Header: h,
	}
	dd = d

	var bs []byte
	if bs, err = i.NextBytesNoCopy(11); err != nil || len(bs) < 11 {
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
	return
}

func (d *TerrestrialDeliverySystem) CalcLength() int {
	return 11
}

func (d *TerrestrialDeliverySystem) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
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
