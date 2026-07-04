package ts

import (
	"errors"

	"github.com/k-danil/go-astits/v2/internal/errclass"
)

// ErrInvalidData is the class of every corrupt-input parse failure across the
// module: errors.Is(err, ErrInvalidData) matches any of them.
var ErrInvalidData = errors.New("astits: invalid data")

var (
	ErrNoMorePackets                = errors.New("astits: no more packets")
	ErrPacketMustStartWithASyncByte = errclass.New("astits: packet must start with a sync byte", ErrInvalidData)
	ErrShortPacket                  = errclass.New("astits: packet too short", ErrInvalidData)
)
