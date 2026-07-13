package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// S2SatelliteDeliverySystem represents an S2 satellite delivery system
// descriptor: DVB-S2 additions signalled alongside the satellite delivery
// system descriptor. ScramblingSequenceIndex is present only when
// ScramblingSequenceSelector, InputStreamIdentifier only when
// MultipleInputStreamFlag.
// Chapter: 6.2.13.3 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type S2SatelliteDeliverySystem struct {
	Header                          Header `json:"_header"`
	ScramblingSequenceIndex         uint32 `json:"scrambling_sequence_index"`
	InputStreamIdentifier           uint8  `json:"input_stream_identifier"`
	ScramblingSequenceSelector      bool   `json:"scrambling_sequence_selector"`
	MultipleInputStreamFlag         bool   `json:"multiple_input_stream_flag"`
	BackwardsCompatibilityIndicator bool   `json:"backwards_compatibility_indicator"`
}

func newDescriptorS2SatelliteDeliverySystem(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	d := &S2SatelliteDeliverySystem{
		Header: h,
	}
	dd = d

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.ScramblingSequenceSelector = b&0x80 > 0
	d.MultipleInputStreamFlag = b&0x40 > 0
	d.BackwardsCompatibilityIndicator = b&0x20 > 0

	if d.ScramblingSequenceSelector {
		var bs []byte
		if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		d.ScramblingSequenceIndex = uint32(bs[0]&0x03)<<16 | uint32(bs[1])<<8 | uint32(bs[2])
	}

	if d.MultipleInputStreamFlag {
		if d.InputStreamIdentifier, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
	}
	return
}

func (d *S2SatelliteDeliverySystem) CalcLength() (n int) {
	n = 1
	if d.ScramblingSequenceSelector {
		n += 3
	}
	if d.MultipleInputStreamFlag {
		n++
	}
	return
}

func (d *S2SatelliteDeliverySystem) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	b := byte(0x1f)
	if d.ScramblingSequenceSelector {
		b |= 0x80
	}
	if d.MultipleInputStreamFlag {
		b |= 0x40
	}
	if d.BackwardsCompatibilityIndicator {
		b |= 0x20
	}
	dst = append(dst, b)
	if d.ScramblingSequenceSelector {
		dst = append(dst,
			0xfc|byte(d.ScramblingSequenceIndex>>16)&0x03,
			byte(d.ScramblingSequenceIndex>>8),
			byte(d.ScramblingSequenceIndex))
	}
	if d.MultipleInputStreamFlag {
		dst = append(dst, d.InputStreamIdentifier)
	}
	return dst
}
