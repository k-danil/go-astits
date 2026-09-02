package ts

import (
	"errors"
	"fmt"
)

// PIDUnset marks a stream-level RecoverableError (sync loss, repaired sync
// byte, dropped packet) that is not bound to a PID; a real PID is 13 bits, so
// 0xFFFF never collides.
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
	default:
		s = "unknown"
	}
	return
}

// RecoverableError is a non-fatal parse failure the demuxer skipped over:
// iteration continues past it. It unwraps to the underlying error, so
// errors.Is(err, ErrInvalidData) and the specific sentinels still match.
type RecoverableError struct {
	Err     error
	Offset  int64 // best-effort: stream byte offset where the failure was detected, not the unit start
	Dropped int64 // bytes lost to this event; 0 is a violation that lost nothing
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

// IsRecoverable reports whether err is a RecoverableError, i.e. a non-terminal
// failure the demuxer skipped so iteration can continue past it.
func IsRecoverable(err error) (ok bool) {
	var re *RecoverableError
	ok = errors.As(err, &re)
	return
}
