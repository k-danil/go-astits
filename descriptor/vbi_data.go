package descriptor

import (
	"fmt"

	"github.com/asticode/go-astikit"
)

// VBI data service id
// Chapter: 6.2.47 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
const (
	VBIDataServiceIDEBUTeletext          = 0x1
	VBIDataServiceIDInvertedTeletext     = 0x2
	VBIDataServiceIDVPS                  = 0x4
	VBIDataServiceIDWSS                  = 0x5
	VBIDataServiceIDClosedCaptioning     = 0x6
	VBIDataServiceIDMonochrome442Samples = 0x7
)

// DescriptorVBIData represents a VBI data descriptor
// Chapter: 6.2.47 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorVBIData struct {
	Header   DescriptorHeader
	Services []DescriptorVBIDataService
}

// DescriptorVBIDataService represents a vbi data service descriptor
// Chapter: 6.2.47 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorVBIDataService struct {
	DataServiceID uint8
	Descriptors   []DescriptorVBIDataDescriptor
}

// DescriptorVBIDataItem represents a vbi data descriptor item
// Chapter: 6.2.47 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type DescriptorVBIDataDescriptor struct {
	FieldParity bool
	LineOffset  uint8
}

func newDescriptorVBIData(i *astikit.BytesIterator, h DescriptorHeader, offsetEnd int) (dd Descriptor, err error) {
	// Create descriptor
	d := &DescriptorVBIData{
		Header:   h,
		Services: make([]DescriptorVBIDataService, (offsetEnd-i.Offset())/3),
	}
	dd = d

	// Loop
	for idx := range d.Services {
		// Get next byte
		var b byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		// Data service ID
		d.Services[idx].DataServiceID = b

		// Get next byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		// Data service descriptor length
		dataServiceDescriptorLength := int(b)

		// Data service descriptor
		offsetDataEnd := i.Offset() + dataServiceDescriptorLength
		for i.Offset() < offsetDataEnd {
			// Get next byte
			if b, err = i.NextByte(); err != nil {
				err = fmt.Errorf("astits: fetching next byte failed: %w", err)
				return
			}

			if d.Services[idx].DataServiceID <= VBIDataServiceIDMonochrome442Samples &&
				d.Services[idx].DataServiceID != 0x0 && d.Services[idx].DataServiceID != 0x3 {
				// Append data
				d.Services[idx].Descriptors = append(d.Services[idx].Descriptors, DescriptorVBIDataDescriptor{
					FieldParity: b&0x20 > 0,
					LineOffset:  b & 0x1f,
				})
			}
		}
	}
	return
}

func (d *DescriptorVBIData) length() uint8 {
	return uint8(3 * len(d.Services))
}

func (d *DescriptorVBIData) write(w *astikit.BitsWriter) (int, error) {
	b := astikit.NewBitsWriterBatch(w)

	length := d.length()
	b.Write(uint8(d.Header.Tag))
	b.Write(length)

	if err := b.Err(); err != nil {
		return 0, err
	}
	written := int(length) + 2

	for _, item := range d.Services {
		b.Write(item.DataServiceID)

		if item.DataServiceID <= VBIDataServiceIDMonochrome442Samples &&
			item.DataServiceID != 0x0 && item.DataServiceID != 0x3 {

			b.Write(uint8(len(item.Descriptors))) // each descriptor is 1 byte
			for _, desc := range item.Descriptors {
				b.WriteN(uint8(0xff), 2)
				b.Write(desc.FieldParity)
				b.WriteN(desc.LineOffset, 5)
			}
		} else {
			// let's put one reserved byte
			b.Write(uint8(1))
			b.Write(uint8(0xff))
		}
	}

	return written, b.Err()
}
