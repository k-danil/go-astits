package dvb

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
	"github.com/k-danil/go-astits/v3/internal/errclass"
	"github.com/k-danil/go-astits/v3/ts"
)

var ErrInvalidBCD = errclass.New("astits: invalid BCD digit", ts.ErrInvalidData)

func ParseTime(i *bytesiter.Iterator) (t time.Time, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(mjdSize); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	mjd := binary.BigEndian.Uint16(bs)
	day := mjdEpoch.Add(time.Duration(mjd) * 24 * time.Hour)

	if bs, err = i.NextBytesNoCopy(hoursMinutesSecondsSize); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	if mjd == undefinedMJD && bs[0] == undefinedBCD && bs[1] == undefinedBCD && bs[2] == undefinedBCD {
		return time.Time{}, nil
	}

	var d time.Duration
	if d, err = parseBCDDuration(bs); err != nil {
		return
	}
	t = day.Add(d)

	return
}

func ParseDurationMinutes(i *bytesiter.Iterator) (d time.Duration, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(hoursMinutesSize); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return parseBCDDuration(bs)
}

func ParseDurationSeconds(i *bytesiter.Iterator) (d time.Duration, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(hoursMinutesSecondsSize); err != nil {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	return parseBCDDuration(bs)
}

var bcdDurationUnits = [...]time.Duration{time.Hour, time.Minute, time.Second}

func parseBCDDuration(bs []byte) (d time.Duration, err error) {
	for idx, b := range bs {
		var n uint8
		if n, err = parseBCDByte(b); err != nil {
			return
		}
		d += time.Duration(n) * bcdDurationUnits[idx]
	}
	return
}

func parseBCDByte(b byte) (n uint8, err error) {
	hi, lo := b>>4, b&0x0f
	if hi > maxBCDDigit || lo > maxBCDDigit {
		err = fmt.Errorf("astits: byte %#02x is not BCD: %w", b, ErrInvalidBCD)
		return
	}
	n = hi*bcdBase + lo
	return
}

var mjdEpoch = time.Date(1858, time.November, 17, 0, 0, 0, 0, time.UTC)

const (
	undefinedMJD = 0xffff
	undefinedBCD = 0xff

	mjdSize                 = 2
	hoursMinutesSize        = 2
	hoursMinutesSecondsSize = 3

	// Saturating onto the marker would read back as no time at all.
	maxMJD      = undefinedMJD - 1
	maxBCDDigit = 9
	maxBCDValue = 99
	bcdBase     = 10

	minutesPerHour   = 60
	secondsPerMinute = 60
)

// Dates outside the representable MJD range (1858-11-17 to 2038-04-21) saturate inside it, clear of the undefined marker.
func AppendTime(dst []byte, t time.Time) []byte {
	if t.IsZero() {
		return append(dst, undefinedMJD>>8, undefinedMJD&0xff, undefinedBCD, undefinedBCD, undefinedBCD)
	}
	t = t.UTC()
	d := t.Sub(t.Truncate(24 * time.Hour))
	mjd := min(max(int(t.Add(-d).Sub(mjdEpoch)/(24*time.Hour)), 0), maxMJD)

	dst = append(dst, byte(mjd>>8), byte(mjd))
	return AppendDurationSeconds(dst, d)
}

func AppendDurationMinutes(dst []byte, d time.Duration) []byte {
	hours, minutes, _ := splitDuration(d)
	return append(dst, bcdByte(hours), bcdByte(minutes))
}

func AppendDurationSeconds(dst []byte, d time.Duration) []byte {
	hours, minutes, seconds := splitDuration(d)
	return append(dst, bcdByte(hours), bcdByte(minutes), bcdByte(seconds))
}

// Hours are two BCD digits: past 99:59:59 it saturates rather than wrap into a short duration.
func splitDuration(d time.Duration) (hours, minutes, seconds int) {
	if d < 0 {
		return
	}
	if hours = int(d / time.Hour); hours > maxBCDValue {
		return maxBCDValue, minutesPerHour - 1, secondsPerMinute - 1
	}
	minutes = int(d/time.Minute) % minutesPerHour
	seconds = int(d/time.Second) % secondsPerMinute
	return
}

func bcdByte(n int) uint8 {
	return uint8(n/bcdBase)<<4 | uint8(n%bcdBase)
}
