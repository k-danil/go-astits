package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// TransportProfile is the MPEG-2 systems Transport_profile_descriptor (ISO/IEC 13818-1).
type TransportProfile struct {
	PrivateData []byte `json:"private_data"`
	Header      Header `json:"_header"`
	Profile     uint8  `json:"transport_profile"`
}

func newDescriptorTransportProfile(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	d := &TransportProfile{
		Header:  h,
		Profile: b,
	}
	dd = d

	if i.Offset() < offsetEnd {
		if d.PrivateData, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *TransportProfile) CalcLength() int {
	return 1 + len(d.PrivateData)
}

func (d *TransportProfile) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, d.Profile)
	return append(dst, d.PrivateData...)
}
