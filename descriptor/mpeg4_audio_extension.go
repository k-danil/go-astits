package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
	"github.com/k-danil/go-astits/v3/internal/util"
)

type MPEG4AudioExtension struct {
	AudioProfileLevelIndication []byte `json:"audioProfileLevelIndication"`
	AudioSpecificConfig         []byte `json:"audioSpecificConfig"`
	Header                      Header `json:"_header"`
	HasASC                      bool   `json:"ASC_flag"`
}

const mpeg4AudioExtensionMaxLoops = 0x0f

// num_of_loops is 4 bits: entries past the fifteenth cannot be signalled, so they are dropped.
func (d *MPEG4AudioExtension) audioProfileLevelIndication() []byte {
	return d.AudioProfileLevelIndication[:min(len(d.AudioProfileLevelIndication), mpeg4AudioExtensionMaxLoops)]
}

func newDescriptorMPEG4AudioExtension(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	d := &MPEG4AudioExtension{
		Header: h,
		HasASC: b&0x80 > 0,
	}
	dd = d

	if numOfLoops := int(b & 0x0f); numOfLoops > 0 {
		if d.AudioProfileLevelIndication, err = i.NextBytes(numOfLoops); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}

	if d.HasASC {
		var ascSize byte
		if ascSize, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		if ascSize > 0 {
			if d.AudioSpecificConfig, err = i.NextBytes(int(ascSize)); err != nil {
				err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
				return
			}
		}
	}

	err = rejectTrailingBytes(i, offsetEnd)
	return
}

func (d *MPEG4AudioExtension) CalcLength() int {
	ret := 1 + len(d.audioProfileLevelIndication())
	if d.HasASC {
		ret += 1 + len(d.AudioSpecificConfig)
	}
	return ret
}

func (d *MPEG4AudioExtension) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	apl := d.audioProfileLevelIndication()
	dst = append(dst, util.B2U(d.HasASC)<<7|0x7<<4|uint8(len(apl)))
	dst = append(dst, apl...)
	if d.HasASC {
		dst = append(dst, uint8(len(d.AudioSpecificConfig)))
		dst = append(dst, d.AudioSpecificConfig...)
	}
	return dst
}
