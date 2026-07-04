package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// Service types
// Chapter: 6.2.33 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
const (
	ServiceTypeDigitalTelevisionService = 0x1
)

// Service represents a service descriptor
// Chapter: 6.2.33 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type Service struct {
	Name     []byte
	Provider []byte
	Header   Header
	Type     uint8
}

func newDescriptorService(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	// Get next byte
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Create descriptor
	d := &Service{
		Header: h,
		Type:   b,
	}
	dd = d

	// Get next byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Provider length
	providerLength := int(b)

	// Provider
	if d.Provider, err = i.NextBytes(providerLength); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	// Get next byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	// Name length
	nameLength := int(b)

	// Name
	if d.Name, err = i.NextBytes(nameLength); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return
}

func (d *Service) CalcLength() int {
	return 3 + len(d.Name) + len(d.Provider) // type and lengths
}

func (d *Service) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, d.Type, uint8(len(d.Provider)))
	dst = append(dst, d.Provider...)
	dst = append(dst, uint8(len(d.Name)))
	return append(dst, d.Name...)
}
