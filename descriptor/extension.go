package descriptor

import (
	"fmt"

	"github.com/asticode/go-astikit"

	"github.com/k-danil/go-astits/internal/util"
)

// Descriptor extension tags
// Chapter: 6.3 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
const (
	DescriptorTagExtensionSupplementaryAudio = 0x6
)

// DescriptorExtension represents an extension descriptor
// Chapter: 6.2.16 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorExtension struct {
	SupplementaryAudio *DescriptorExtensionSupplementaryAudio
	Unknown            []byte
	Header             DescriptorHeader
	Tag                uint8
}

func newDescriptorExtension(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (dd Descriptor, err error) {
	// Get next byte
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Create descriptor
	d := &DescriptorExtension{
		Header: h,
		Tag:    b,
	}
	dd = d

	// Switch on tag
	switch d.Tag {
	case DescriptorTagExtensionSupplementaryAudio:
		if d.SupplementaryAudio, err = newDescriptorExtensionSupplementaryAudio(i, offsetEnd); err != nil {
			err = fmt.Errorf("astits: parsing extension supplementary audio descriptor failed: %w", err)
			return
		}
	default:
		if d.Unknown, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *DescriptorExtension) length() uint8 {
	ret := 1 // tag

	switch d.Tag {
	case DescriptorTagExtensionSupplementaryAudio:
		ret += d.SupplementaryAudio.length()
	default:
		ret += len(d.Unknown)
	}

	return uint8(ret)
}

func (d *DescriptorExtension) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	b.Write(d.Tag)

	switch d.Tag {
	case DescriptorTagExtensionSupplementaryAudio:
		err := d.SupplementaryAudio.write(w)
		if err != nil {
			return 0, err
		}
	default:
		if len(d.Unknown) > 0 {
			b.Write(d.Unknown)
		}
	}

	return written, b.Err()
}

// DescriptorExtensionSupplementaryAudio represents a supplementary audio extension descriptor
// Chapter: 6.4.10 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorExtensionSupplementaryAudio struct {
	PrivateData             []byte
	LanguageCode            [3]byte
	EditorialClassification uint8
	HasLanguageCode         bool
	MixType                 bool
}

func newDescriptorExtensionSupplementaryAudio(i *astikit.BytesIterator, offsetEnd int) (d *DescriptorExtensionSupplementaryAudio, err error) {
	// Get next byte
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Init
	d = &DescriptorExtensionSupplementaryAudio{
		EditorialClassification: b >> 2 & 0x1f,
		HasLanguageCode:         b&0x1 > 0,
		MixType:                 b&0x80 > 0,
	}

	// Language code
	if d.HasLanguageCode {
		var bs []byte
		if bs, err = i.NextBytesNoCopy(3); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		copy(d.LanguageCode[:], bs)
	}

	// Private data
	if i.Offset() < offsetEnd {
		if d.PrivateData, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *DescriptorExtensionSupplementaryAudio) length() int {
	ret := 1
	ret += 3 * int(util.B2U(d.HasLanguageCode))
	ret += len(d.PrivateData)
	return ret
}

func (d *DescriptorExtensionSupplementaryAudio) write(w *astikit.BitsWriter) error {
	b := astikit.NewBitsWriterBatch(w)

	b.Write(d.MixType)
	b.WriteN(d.EditorialClassification, 5)
	b.Write(true) // reserved
	b.Write(d.HasLanguageCode)

	if d.HasLanguageCode {
		b.Write(d.LanguageCode[:])
	}

	b.Write(d.PrivateData)

	return b.Err()
}
