// Package mux writes an MPEG-TS stream. [New] builds a [Muxer]; register
// elementary streams with [Muxer.AddElementaryStream], then emit PES units with
// [Muxer.WriteData] and PSI tables with [Muxer.WriteTables]. Already-formed
// packets pass straight through [Muxer.WritePacket], writing [ts.Packet.Raw]
// when available and reserializing otherwise.
//
// A muxer is single-goroutine and holds no locks. Fixed-size serialization
// panics on a short buffer; see the module documentation.
package mux
