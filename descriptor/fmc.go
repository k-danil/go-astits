package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// FMC is the MPEG-2 systems FMC_descriptor (ISO/IEC 13818-1).
type FMC struct {
	Entries []FMCEntry
	Header  Header
}

type FMCEntry struct {
	ESID           uint16
	FlexMuxChannel uint8
}

func newDescriptorFMC(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	d := &FMC{
		Header: h,
	}
	dd = d

	for i.Offset() < offsetEnd {
		var bs []byte
		if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}

		d.Entries = append(d.Entries, FMCEntry{
			ESID:           binary.BigEndian.Uint16(bs),
			FlexMuxChannel: bs[2],
		})
	}
	return
}

func (d *FMC) CalcLength() int {
	return 3 * len(d.Entries)
}

func (d *FMC) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	for _, e := range d.Entries {
		dst = append(dst, byte(e.ESID>>8), byte(e.ESID), e.FlexMuxChannel)
	}
	return dst
}
