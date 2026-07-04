package ts

import "errors"

var (
	ErrNoMorePackets                = errors.New("astits: no more packets")
	ErrPacketMustStartWithASyncByte = errors.New("astits: packet must start with a sync byte")
	ErrShortPacket                  = errors.New("astits: packet too short")
)
