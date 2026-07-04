package dvb

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// ParseTime parses a DVB time
// This field is coded as 16 bits giving the 16 LSBs of MJD followed by 24 bits coded as 6 digits in 4 - bit Binary
// Coded Decimal (BCD). If the start time is undefined (e.g. for an event in a NVOD reference service) all bits of the
// field are set to "1".
// I apologize for the computation which is really messy but details are given in the documentation
// Page: 160 | Annex C | Link: https://www.dvb.org/resources/public/standards/a38_dvb-si_specification.pdf
// (barbashov) the link above can be broken, alternative: https://dvb.org/wp-content/uploads/2019/12/a038_tm1217r37_en300468v1_17_1_-_rev-134_-_si_specification.pdf
func ParseTime(i *bytesiter.Iterator) (t time.Time, err error) {
	// Get next 2 bytes
	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	// Date
	day := mjdEpoch.Add(time.Duration(binary.BigEndian.Uint16(bs)) * 24 * time.Hour)

	if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	t = day.Add(time.Duration(parseDurationByte(bs[0]))*time.Hour +
		time.Duration(parseDurationByte(bs[1]))*time.Minute +
		time.Duration(parseDurationByte(bs[2]))*time.Second)

	return
}

// ParseDurationMinutes parses a minutes duration
// 16 bit field containing the duration of the event in hours, minutes. format: 4 digits, 4 - bit BCD = 18 bit
func ParseDurationMinutes(i *bytesiter.Iterator) (d time.Duration, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d = parseDurationByte(bs[0])*time.Hour + parseDurationByte(bs[1])*time.Minute
	return
}

// ParseDurationSeconds parses a seconds duration
// 24 bit field containing the duration of the event in hours, minutes, seconds. format: 6 digits, 4 - bit BCD = 24 bit
func ParseDurationSeconds(i *bytesiter.Iterator) (d time.Duration, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d = parseDurationByte(bs[0])*time.Hour + parseDurationByte(bs[1])*time.Minute + parseDurationByte(bs[2])*time.Second
	return
}

// parseDurationByte parses a duration byte
func parseDurationByte(i byte) time.Duration {
	return time.Duration(i>>4*10 + i&0xf)
}

// mjdEpoch is 1858-11-17 UTC, day zero of the Modified Julian Date scale.
var mjdEpoch = time.Date(1858, time.November, 17, 0, 0, 0, 0, time.UTC)

func AppendTime(dst []byte, t time.Time) []byte {
	t = t.UTC()
	d := t.Sub(t.Truncate(24 * time.Hour))
	mjd := int(t.Add(-d).Sub(mjdEpoch) / (24 * time.Hour))

	dst = append(dst, byte(mjd>>8), byte(mjd))
	return AppendDurationSeconds(dst, d)
}

func AppendDurationMinutes(dst []byte, d time.Duration) []byte {
	hours := uint8(d.Hours())
	minutes := uint8(int(d.Minutes()) % 60)

	return append(dst, durationByteRepresentation(hours), durationByteRepresentation(minutes))
}

func AppendDurationSeconds(dst []byte, d time.Duration) []byte {
	hours := uint8(d.Hours())
	minutes := uint8(int(d.Minutes()) % 60)
	seconds := uint8(int(d.Seconds()) % 60)

	return append(dst, durationByteRepresentation(hours), durationByteRepresentation(minutes), durationByteRepresentation(seconds))
}

func durationByteRepresentation(n uint8) uint8 {
	return (n/10)<<4 | n%10
}
