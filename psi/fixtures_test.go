package psi

import (
	"time"

	"github.com/asticode/go-astikit"

	"github.com/k-danil/go-astits/descriptor"
)

var descriptors = []descriptor.Descriptor{
	&descriptor.DescriptorStreamIdentifier{
		Header: descriptor.DescriptorHeader{
			Tag:    descriptor.DescriptorTagStreamIdentifier,
			Length: 0x1,
		},
		ComponentTag: 0x7},
}

func descriptorsBytes(w *astikit.BitsWriter) {
	w.Write("000000000011")                                  // Overall length
	w.Write(uint8(descriptor.DescriptorTagStreamIdentifier)) // Tag
	w.Write(uint8(1))                                        // Length
	w.Write(uint8(7))                                        // Component tag
}

var (
	dvbDurationSeconds      = time.Hour + 45*time.Minute + 30*time.Second
	dvbDurationSecondsBytes = []byte{0x1, 0x45, 0x30} // 014530
	dvbTime, _              = time.Parse("2006-01-02 15:04:05", "1993-10-13 12:45:00")
	dvbTimeBytes            = []byte{0xc0, 0x79, 0x12, 0x45, 0x0} // C079124500
)
