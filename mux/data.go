package mux

import (
	"github.com/k-danil/go-astits/v3/pes"
	"github.com/k-danil/go-astits/v3/ts"
)

type Data struct {
	PID             uint16
	AdaptationField *ts.PacketAdaptationField
	PES             *pes.Data
}
