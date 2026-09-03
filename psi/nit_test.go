package psi

import (
	"bytes"

	"github.com/k-danil/go-astits/v3/internal/bitstest"
)

var nit = &NIT{
	NetworkDescriptors: descriptors,
	NetworkID:          1,
	TransportStreams: []NITTransportStream{{
		OriginalNetworkID:    3,
		TransportDescriptors: descriptors,
		TransportStreamID:    2,
	}},
}

func nitBytes() []byte {
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	_ = w.Write("0000")         // Reserved for future use
	descriptorsBytes(w)         // Network descriptors
	_ = w.Write("0000")         // Reserved for future use
	_ = w.Write("000000001001") // Transport stream loop length
	_ = w.Write(uint16(2))      // Transport stream #1 id
	_ = w.Write(uint16(3))      // Transport stream #1 original network id
	_ = w.Write("0000")         // Transport stream #1 reserved for future use
	descriptorsBytes(w)         // Transport stream #1 descriptors
	return buf.Bytes()
}
