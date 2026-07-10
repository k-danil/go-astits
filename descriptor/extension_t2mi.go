package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// ExtensionT2MI represents a T2-MI extension descriptor: identifies a PID
// carrying a single T2-MI stream.
// Chapter: 6.4.13 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type ExtensionT2MI struct {
	Reserved               []byte
	T2MIStreamID           uint8
	NumT2MIStreamsMinusOne uint8
	PCRISCRCommonClockFlag bool
}

func newDescriptorExtensionT2MI(i *bytesiter.Iterator, offsetEnd int) (d *ExtensionT2MI, err error) {
	d = &ExtensionT2MI{}

	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d.T2MIStreamID = bs[0] & 0x07
	d.NumT2MIStreamsMinusOne = bs[1] & 0x07
	d.PCRISCRCommonClockFlag = bs[2]&0x01 > 0

	if d.Reserved, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *ExtensionT2MI) CalcLength() int {
	return 3 + len(d.Reserved)
}

func (d *ExtensionT2MI) Append(dst []byte) []byte {
	var pcr byte
	if d.PCRISCRCommonClockFlag {
		pcr = 0x01
	}
	// Table 149 NOTE: the reserved_future_use bits shall all be 0.
	dst = append(dst, d.T2MIStreamID&0x07, d.NumT2MIStreamsMinusOne&0x07, pcr)
	return append(dst, d.Reserved...)
}
