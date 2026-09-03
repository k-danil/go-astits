package dvb

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

func ParseTime(i *bytesiter.Iterator) (t time.Time, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	mjd := binary.BigEndian.Uint16(bs)
	day := mjdEpoch.Add(time.Duration(mjd) * 24 * time.Hour)

	if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	if mjd == undefinedMJD && bs[0] == undefinedBCD && bs[1] == undefinedBCD && bs[2] == undefinedBCD {
		return time.Time{}, nil
	}
	t = day.Add(time.Duration(parseDurationByte(bs[0]))*time.Hour +
		time.Duration(parseDurationByte(bs[1]))*time.Minute +
		time.Duration(parseDurationByte(bs[2]))*time.Second)

	return
}

func ParseDurationMinutes(i *bytesiter.Iterator) (d time.Duration, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d = parseDurationByte(bs[0])*time.Hour + parseDurationByte(bs[1])*time.Minute
	return
}

func ParseDurationSeconds(i *bytesiter.Iterator) (d time.Duration, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(3); err != nil || len(bs) < 3 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}
	d = parseDurationByte(bs[0])*time.Hour + parseDurationByte(bs[1])*time.Minute + parseDurationByte(bs[2])*time.Second
	return
}

func parseDurationByte(i byte) time.Duration {
	return time.Duration(i>>4*10 + i&0xf)
}

var mjdEpoch = time.Date(1858, time.November, 17, 0, 0, 0, 0, time.UTC)

const (
	undefinedMJD = 0xffff
	undefinedBCD = 0xff
)

func AppendTime(dst []byte, t time.Time) []byte {
	if t.IsZero() {
		return append(dst, undefinedMJD>>8, undefinedMJD&0xff, undefinedBCD, undefinedBCD, undefinedBCD)
	}
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
