package descriptor

import (
	"fmt"

	"github.com/asticode/go-astikit"

	"github.com/k-danil/go-astits/internal/util"
)

// DescriptorAC3 represents an AC3 descriptor
// Chapter: Annex D | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorAC3 struct {
	Header           DescriptorHeader
	AdditionalInfo   []byte
	ASVC             uint8
	BSID             uint8
	ComponentType    uint8
	HasASVC          bool
	HasBSID          bool
	HasComponentType bool
	HasMainID        bool
	MainID           uint8
}

func newDescriptorAC3(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (dd Descriptor, err error) {
	// Get next byte
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Create descriptor
	d := &DescriptorAC3{
		Header:           h,
		HasASVC:          uint8(b&0x10) > 0,
		HasBSID:          uint8(b&0x40) > 0,
		HasComponentType: uint8(b&0x80) > 0,
		HasMainID:        uint8(b&0x20) > 0,
	}
	dd = d

	// Component type
	if d.HasComponentType {
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.ComponentType = b
	}

	// BSID
	if d.HasBSID {
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.BSID = b
	}

	// Main ID
	if d.HasMainID {
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.MainID = b
	}

	// ASVC
	if d.HasASVC {
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.ASVC = b
	}

	// Additional info
	if i.Offset() < offsetEnd {
		if d.AdditionalInfo, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *DescriptorAC3) length() (ret uint8) {
	ret = 1 // flags
	ret += util.B2U(d.HasComponentType)
	ret += util.B2U(d.HasBSID)
	ret += util.B2U(d.HasMainID)
	ret += util.B2U(d.HasASVC)
	ret += uint8(len(d.AdditionalInfo))

	return ret
}

func (d *DescriptorAC3) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.HasComponentType)
	b.Write(d.HasBSID)
	b.Write(d.HasMainID)
	b.Write(d.HasASVC)
	b.WriteN(uint8(0xff), 4)

	if d.HasComponentType {
		b.Write(d.ComponentType)
	}
	if d.HasBSID {
		b.Write(d.BSID)
	}
	if d.HasMainID {
		b.Write(d.MainID)
	}
	if d.HasASVC {
		b.Write(d.ASVC)
	}
	b.Write(d.AdditionalInfo)

	return written, b.Err()
}
