package ts

import (
	"errors"
	"fmt"
)

// A real PID is 13 bits, so 0xFFFF never collides.
const PIDUnset uint16 = 0xFFFF

type ErrorKind uint8

const (
	ErrorKindSyncLoss ErrorKind = iota
	ErrorKindPacketDrop
	ErrorKindCRC
	ErrorKindPSI
	ErrorKindPES
	ErrorKindTornUnit
	ErrorKindUnknownUnit
	ErrorKindSyncByte
	ErrorKindContinuity
)

func (k ErrorKind) String() (s string) {
	switch k {
	case ErrorKindSyncLoss:
		s = "sync-loss"
	case ErrorKindPacketDrop:
		s = "packet-drop"
	case ErrorKindCRC:
		s = "crc"
	case ErrorKindPSI:
		s = "psi"
	case ErrorKindPES:
		s = "pes"
	case ErrorKindTornUnit:
		s = "torn-unit"
	case ErrorKindUnknownUnit:
		s = "unknown-unit"
	case ErrorKindSyncByte:
		s = "sync-byte"
	case ErrorKindContinuity:
		s = "continuity"
	default:
		s = "unknown"
	}
	return
}

// A parse failure the demuxer skipped; iteration continues past it.
type RecoverableError struct {
	Err     error
	Offset  int64 // where the failure was detected, not the unit start
	Dropped int64 // 0 = a violation that lost nothing, not "unknown"
	Kind    ErrorKind
	PID     uint16
}

func (e *RecoverableError) Error() (s string) {
	if e.PID == PIDUnset {
		s = fmt.Sprintf("astits: recoverable %s error at offset %d, %d bytes dropped: %v", e.Kind, e.Offset, e.Dropped, e.Err)
	} else {
		s = fmt.Sprintf("astits: recoverable %s error on PID %d at offset %d, %d bytes dropped: %v", e.Kind, e.PID, e.Offset, e.Dropped, e.Err)
	}
	return
}

func (e *RecoverableError) Unwrap() error { return e.Err }

func IsRecoverable(err error) (ok bool) {
	_, ok = errors.AsType[*RecoverableError](err)
	return
}
