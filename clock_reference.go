package astits

import (
	"time"
)

// ClockReference represents a clock reference
// Base is based on a 90 kHz clock and extension is based on a 27 MHz clock
type ClockReference struct {
	Base, Extension int64
}

// newClockReference builds a new clock reference
func newClockReference(base, extension int64) ClockReference {
	return ClockReference{
		Base:      base,
		Extension: extension,
	}
}

// Duration converts the clock reference into duration
func (cr *ClockReference) Duration() time.Duration {
	return time.Duration(cr.Base*1e9/90000) + time.Duration(cr.Extension*1e9/27000000)
}

// Time converts the clock reference into time
func (cr *ClockReference) Time() time.Time {
	return time.Unix(0, cr.Duration().Nanoseconds())
}
