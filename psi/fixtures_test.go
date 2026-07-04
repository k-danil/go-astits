package psi

import (
	"time"

	"github.com/k-danil/go-astits/v2/descriptor"
	"github.com/k-danil/go-astits/v2/internal/bitstest"
)

var descriptors = []descriptor.Descriptor{
	&descriptor.StreamIdentifier{
		Header: descriptor.Header{
			Tag:    descriptor.TagStreamIdentifier,
			Length: 0x1,
		},
		ComponentTag: 0x7},
}

func descriptorsBytes(w *bitstest.Writer) {
	_ = w.Write("000000000011")                        // Overall length
	_ = w.Write(uint8(descriptor.TagStreamIdentifier)) // Tag
	_ = w.Write(uint8(1))                              // Length
	_ = w.Write(uint8(7))                              // Component tag
}

var (
	dvbDurationSeconds      = time.Hour + 45*time.Minute + 30*time.Second
	dvbDurationSecondsBytes = []byte{0x1, 0x45, 0x30} // 014530
	dvbTime, _              = time.Parse("2006-01-02 15:04:05", "1993-10-13 12:45:00")
	dvbTimeBytes            = []byte{0xc0, 0x79, 0x12, 0x45, 0x0} // C079124500
)
