package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
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

// VBIData represents a VBI data descriptor
// Chapter: 6.2.47 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type VBIData struct {
	Header   Header
	Services []VBIDataService
}

// VBIDataService represents a vbi data service descriptor
// Chapter: 6.2.47 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type VBIDataService struct {
	DataServiceID uint8
	Descriptors   []VBIDataDescriptor
}

// DescriptorVBIDataItem represents a vbi data descriptor item
// Chapter: 6.2.47 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type VBIDataDescriptor struct {
	FieldParity bool
	LineOffset  uint8
}

func newDescriptorVBIData(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	// Create descriptor
	d := &VBIData{Header: h}
	dd = d

	// Loop: services are variable-size (id, length, payload)
	for i.Offset() < offsetEnd {
		var svc VBIDataService

		// Get next byte
		var b byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		// Data service ID
		svc.DataServiceID = b

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

			if svc.DataServiceID <= VBIDataServiceIDMonochrome442Samples &&
				svc.DataServiceID != 0x0 && svc.DataServiceID != 0x3 {
				// Append data
				svc.Descriptors = append(svc.Descriptors, VBIDataDescriptor{
					FieldParity: b&0x20 > 0,
					LineOffset:  b & 0x1f,
				})
			}
		}

		d.Services = append(d.Services, svc)
	}
	return
}

func (d *VBIData) CalcLength() int {
	var ret int
	for _, item := range d.Services {
		ret += 2 // service id and length
		if item.DataServiceID <= VBIDataServiceIDMonochrome442Samples &&
			item.DataServiceID != 0x0 && item.DataServiceID != 0x3 {
			ret += len(item.Descriptors) // each descriptor is 1 byte
		} else {
			ret++ // one reserved byte
		}
	}
	return ret
}

func (d *VBIData) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	for _, item := range d.Services {
		dst = append(dst, item.DataServiceID)

		if item.DataServiceID <= VBIDataServiceIDMonochrome442Samples &&
			item.DataServiceID != 0x0 && item.DataServiceID != 0x3 {

			dst = append(dst, uint8(len(item.Descriptors))) // each descriptor is 1 byte
			for _, desc := range item.Descriptors {
				dst = append(dst, 0xc0|util.B2U(desc.FieldParity)<<5|desc.LineOffset&0x1f)
			}
		} else {
			// let's put one reserved byte
			dst = append(dst, uint8(1), 0xff)
		}
	}
	return dst
}
