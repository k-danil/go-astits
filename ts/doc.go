// Package ts reads and writes MPEG-TS packets: the 188/192-byte packet, its
// header and adaptation field, the clock references (PCR/PTS/DTS/ESCR), CRC32,
// and the packet reader with copy and zero-copy view modes.
//
// Packets are pooled: obtain one with [NewPacket] and return it with
// [Packet.Close]. In zero-copy view mode a packet's bytes ([Packet.Raw],
// [Packet.Payload], and an adaptation field's TransportPrivateData) view a
// shared buffer valid only until the next read; copy anything you keep. See the
// module documentation for the full lifetime and serialization contracts.
package ts
