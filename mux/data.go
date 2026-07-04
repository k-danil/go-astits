package mux

import (
	"github.com/k-danil/go-astits/pes"
	"github.com/k-danil/go-astits/ts"
)

// Data represents a data to be written by Muxer
type Data struct {
	PID             uint16
	AdaptationField *ts.PacketAdaptationField
	PES             *pes.Data
}
