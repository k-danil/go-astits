package mux

import (
	"github.com/k-danil/go-astits/v2/pes"
	"github.com/k-danil/go-astits/v2/ts"
)

// Data represents a data to be written by Muxer
type Data struct {
	PID             uint16
	AdaptationField *ts.PacketAdaptationField
	PES             *pes.Data
}
