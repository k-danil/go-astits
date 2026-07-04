package descriptor

import (
	"fmt"

	"github.com/asticode/go-astikit"

	"github.com/k-danil/go-astits/internal/util"
)

// DescriptorEnhancedAC3 represents an enhanced AC3 descriptor
// Chapter: Annex D | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorEnhancedAC3 struct {
	Header           DescriptorHeader
	AdditionalInfo   []byte
	ASVC             uint8
	BSID             uint8
	ComponentType    uint8
	HasASVC          bool
	HasBSID          bool
	HasComponentType bool
	HasMainID        bool
	HasSubStream1    bool
	HasSubStream2    bool
	HasSubStream3    bool
	MainID           uint8
	MixInfoExists    bool
	SubStream1       uint8
	SubStream2       uint8
	SubStream3       uint8
}

func newDescriptorEnhancedAC3(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (dd Descriptor, err error) {
	// Get next byte
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Create descriptor
	d := &DescriptorEnhancedAC3{
		Header:           h,
		HasASVC:          uint8(b&0x10) > 0,
		HasBSID:          uint8(b&0x40) > 0,
		HasComponentType: uint8(b&0x80) > 0,
		HasMainID:        uint8(b&0x20) > 0,
		HasSubStream1:    uint8(b&0x4) > 0,
		HasSubStream2:    uint8(b&0x2) > 0,
		HasSubStream3:    uint8(b&0x1) > 0,
		MixInfoExists:    uint8(b&0x8) > 0,
	}
	dd = d

	// Component type
	if d.HasComponentType {
		// Get next byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.ComponentType = b
	}

	// BSID
	if d.HasBSID {
		// Get next byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.BSID = b
	}

	// Main ID
	if d.HasMainID {
		// Get next byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.MainID = b
	}

	// ASVC
	if d.HasASVC {
		// Get next byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.ASVC = b
	}

	// Substream 1
	if d.HasSubStream1 {
		// Get next byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.SubStream1 = b
	}

	// Substream 2
	if d.HasSubStream2 {
		// Get next byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.SubStream2 = b
	}

	// Substream 3
	if d.HasSubStream3 {
		// Get next byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.SubStream3 = b
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

func (d *DescriptorEnhancedAC3) length() (ret uint8) {
	ret = 1 // flags
	ret += util.B2U(d.HasComponentType)
	ret += util.B2U(d.HasBSID)
	ret += util.B2U(d.HasMainID)
	ret += util.B2U(d.HasASVC)
	ret += util.B2U(d.HasSubStream1)
	ret += util.B2U(d.HasSubStream2)
	ret += util.B2U(d.HasSubStream3)
	ret += uint8(len(d.AdditionalInfo))

	return
}

func (d *DescriptorEnhancedAC3) write(w *astikit.BitsWriter) (int, error) {
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
	b.Write(d.MixInfoExists)
	b.Write(d.HasSubStream1)
	b.Write(d.HasSubStream2)
	b.Write(d.HasSubStream3)

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
	if d.HasSubStream1 {
		b.Write(d.SubStream1)
	}
	if d.HasSubStream2 {
		b.Write(d.SubStream2)
	}
	if d.HasSubStream3 {
		b.Write(d.SubStream3)
	}

	b.Write(d.AdditionalInfo)

	return written, b.Err()
}
