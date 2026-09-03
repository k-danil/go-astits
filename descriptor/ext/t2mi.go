package ext

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type T2MI struct {
	Reserved               []byte `json:"_reserved"`
	T2MIStreamID           uint8  `json:"t2mi_stream_id"`
	NumT2MIStreamsMinusOne uint8  `json:"num_t2mi_streams_minus_one"`
	PCRISCRCommonClockFlag bool   `json:"pcr_iscr_common_clock_flag"`
}

func parseT2MI(i *bytesiter.Iterator, offsetEnd int) (d *T2MI, err error) {
	d = &T2MI{}

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

func (d *T2MI) CalcLength() int {
	return 3 + len(d.Reserved)
}

func (d *T2MI) Append(dst []byte) []byte {
	var pcr byte
	if d.PCRISCRCommonClockFlag {
		pcr = 0x01
	}
	// Table 149: these reserved_future_use bits are 0, unlike the 1-padding elsewhere in this package.
	dst = append(dst, d.T2MIStreamID&0x07, d.NumT2MIStreamsMinusOne&0x07, pcr)
	return append(dst, d.Reserved...)
}
