package ts

import (
	"errors"

	"github.com/k-danil/go-astits/v3/internal/errclass"
)

var ErrInvalidData = errors.New("astits: invalid data")

var (
	ErrNoMorePackets                  = errors.New("astits: no more packets")
	ErrPacketMustStartWithASyncByte   = errclass.New("astits: packet must start with a sync byte", ErrInvalidData)
	ErrShortPacket                    = errclass.New("astits: packet too short", ErrInvalidData)
	ErrAdaptationFieldOverflow        = errclass.New("astits: adaptation field longer than 255 bytes", ErrInvalidData)
	ErrContradictoryAdaptationField   = errclass.New("astits: contradictory adaptation field", ErrInvalidData)
	ErrReservedAdaptationFieldControl = errclass.New("astits: reserved adaptation_field_control", ErrInvalidData)
	ErrContinuityGap                  = errclass.New("astits: continuity counter gap", ErrInvalidData)
	ErrDiscontinuity                  = errclass.New("astits: discontinuity indicator", ErrInvalidData)
	ErrTransportError                 = errclass.New("astits: transport error indicator", ErrInvalidData)
	ErrScrambled                      = errclass.New("astits: scrambled payload", ErrInvalidData)
	ErrUnknownPayload                 = errclass.New("astits: unknown payload", ErrInvalidData)
	ErrUnitTooLarge                   = errclass.New("astits: payload unit exceeds the size limit", ErrInvalidData)
	ErrDuplicateMismatch              = errclass.New("astits: duplicate packet differs from the original", ErrInvalidData)
	ErrHeadlessUnit                   = errclass.New("astits: payload unit without its start", ErrInvalidData)
	ErrSyncLoss                       = errclass.New("astits: sync loss", ErrInvalidData)
)
