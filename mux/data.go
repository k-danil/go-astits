package mux

import (
	"github.com/k-danil/go-astits/pes"
	"github.com/k-danil/go-astits/ts"
)

// MuxerData represents a data to be written by Muxer
type MuxerData struct {
	PID             uint16
	AdaptationField *ts.PacketAdaptationField
	PES             *pes.PESData
}
