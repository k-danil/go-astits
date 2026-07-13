package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// FlexMuxTiming is the MPEG-2 systems FlexMuxTiming_descriptor (ISO/IEC 13818-1).
type FlexMuxTiming struct {
	FCRResolution uint32 `json:"FCRResolution"` // cycles per second
	Header        Header `json:"_header"`
	FCRESID       uint16 `json:"FCR_ES_ID"`
	FCRLength     uint8  `json:"FCRLength"`
	FmxRateLength uint8  `json:"FmxRateLength"`
}

func newDescriptorFlexMuxTiming(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(8); err != nil || len(bs) < 8 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &FlexMuxTiming{
		Header:        h,
		FCRESID:       binary.BigEndian.Uint16(bs),
		FCRResolution: binary.BigEndian.Uint32(bs[2:]),
		FCRLength:     bs[6],
		FmxRateLength: bs[7],
	}
	dd = d
	return
}

func (*FlexMuxTiming) CalcLength() int {
	return 8
}

func (d *FlexMuxTiming) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = binary.BigEndian.AppendUint16(dst, d.FCRESID)
	dst = binary.BigEndian.AppendUint32(dst, d.FCRResolution)
	return append(dst, d.FCRLength, d.FmxRateLength)
}
