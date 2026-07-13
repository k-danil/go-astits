package descriptor

import (
	"encoding/binary"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// ExternalESID is the MPEG-2 systems External_ES_ID_descriptor (ISO/IEC 13818-1).
type ExternalESID struct {
	Header       Header `json:"_header"`
	ExternalESID uint16 `json:"External_ES_ID"`
}

func newDescriptorExternalESID(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &ExternalESID{
		Header:       h,
		ExternalESID: binary.BigEndian.Uint16(bs),
	}
	dd = d
	return
}

func (*ExternalESID) CalcLength() int {
	return 2
}

func (d *ExternalESID) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, byte(d.ExternalESID>>8), byte(d.ExternalESID))
}
