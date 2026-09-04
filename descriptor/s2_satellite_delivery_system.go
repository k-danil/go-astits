package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

// S2SatelliteDeliverySystem carries ScramblingSequenceIndex only when
// ScramblingSequenceSelector is set, InputStreamIdentifier only when
// MultipleInputStreamFlag is, and TimesliceNumber only when NotTimesliceFlag
// is clear.
type S2SatelliteDeliverySystem struct {
	Header                     Header `json:"_header"`
	ScramblingSequenceIndex    uint32 `json:"scrambling_sequence_index"`
	InputStreamIdentifier      uint8  `json:"input_stream_identifier"`
	TimesliceNumber            uint8  `json:"timeslice_number"`
	TSGSMode                   uint8  `json:"TS_GS_mode"`
	ScramblingSequenceSelector bool   `json:"scrambling_sequence_selector"`
	MultipleInputStreamFlag    bool   `json:"multiple_input_stream_flag"`
	NotTimesliceFlag           bool   `json:"not_timeslice_flag"`
}

const (
	s2ScramblingSequenceIndexSize = 3
	s2ReservedFutureUse           = 0x0c
)

func newDescriptorS2SatelliteDeliverySystem(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
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
	d.NotTimesliceFlag = b&0x10 > 0
	d.TSGSMode = b & 0x03

	if d.ScramblingSequenceSelector {
		var bs []byte
		if bs, err = i.NextBytesNoCopy(s2ScramblingSequenceIndexSize); err != nil {
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

	if !d.NotTimesliceFlag {
		if d.TimesliceNumber, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
	}

	err = rejectTrailingBytes(i, offsetEnd)
	return
}

func (d *S2SatelliteDeliverySystem) CalcLength() (n int) {
	n = 1
	if d.ScramblingSequenceSelector {
		n += s2ScramblingSequenceIndexSize
	}
	if d.MultipleInputStreamFlag {
		n++
	}
	if !d.NotTimesliceFlag {
		n++
	}
	return n
}

func (d *S2SatelliteDeliverySystem) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	b := byte(s2ReservedFutureUse) | d.TSGSMode&0x03
	if d.ScramblingSequenceSelector {
		b |= 0x80
	}
	if d.MultipleInputStreamFlag {
		b |= 0x40
	}
	if d.NotTimesliceFlag {
		b |= 0x10
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
	if !d.NotTimesliceFlag {
		dst = append(dst, d.TimesliceNumber)
	}
	return dst
}
