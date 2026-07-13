package psi

import (
	"fmt"
	"time"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/dvb"
)

// TDT represents a TDT: the time and date table carries only the current UTC,
// without descriptors or a CRC (unlike the TOT).
// Page: 39 | Chapter: 5.2.5 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type TDT struct {
	UTCTime time.Time `json:"UTC_time"`
}

// dvbTimeBytesSize is the UTC_time field: 16-bit MJD + 24-bit BCD (§annex C).
const dvbTimeBytesSize = 5

// parseTDTSection parses a TDT section
func parseTDTSection(i *bytesiter.Iterator) (d *TDT, err error) {
	d = &TDT{}
	if d.UTCTime, err = dvb.ParseTime(i); err != nil {
		err = fmt.Errorf("astits: parsing DVB time failed: %w", err)
		return
	}
	return
}

func (d *TDT) CalcSectionLength() int { return dvbTimeBytesSize }

func (d *TDT) appendSection(dst []byte) []byte { return dvb.AppendTime(dst, d.UTCTime) }
