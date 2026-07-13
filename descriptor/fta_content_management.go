package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// FTAContentManagement represents an FTA content management descriptor: the
// content-management policy (scrambling, remote access, revocation) for a
// free-to-air DVB service item.
// Chapter: 6.2.18 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type FTAContentManagement struct {
	Header                          Header `json:"_header"`
	ControlRemoteAccessOverInternet uint8  `json:"control_remote_access_over_internet"`
	UserDefined                     bool   `json:"user_defined"`
	DoNotScramble                   bool   `json:"do_not_scramble"`
	DoNotApplyRevocation            bool   `json:"do_not_apply_revocation"`
}

func newDescriptorFTAContentManagement(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	d := &FTAContentManagement{
		Header: h,
	}
	dd = d

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.UserDefined = b&0x80 > 0
	d.DoNotScramble = b&0x08 > 0
	d.ControlRemoteAccessOverInternet = b >> 1 & 0x03
	d.DoNotApplyRevocation = b&0x01 > 0
	return
}

func (d *FTAContentManagement) CalcLength() int {
	return 1
}

func (d *FTAContentManagement) Append(dst []byte) []byte {
	b := byte(0x70) | d.ControlRemoteAccessOverInternet&0x03<<1
	if d.UserDefined {
		b |= 0x80
	}
	if d.DoNotScramble {
		b |= 0x08
	}
	if d.DoNotApplyRevocation {
		b |= 0x01
	}
	return append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()), b)
}
